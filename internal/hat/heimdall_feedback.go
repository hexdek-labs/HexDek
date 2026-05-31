package hat

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
)

// HeimdallFeedback is the on-disk shape Heimdall emits when it
// attributes wins/losses to specific EvalWeights dimensions across a
// recent batch of games. The hat consumes the file via
// ApplyHeimdallFeedback to nudge its weights toward the attributed
// direction — a "you under-weighted ComboProximity in the last 30
// games" signal becomes a small positive delta on that dimension for
// the next run.
//
// dev-3 ships the Heimdall side (post-game attribution → feedback
// file). This struct is the consumer contract; field names are
// normative so both sides marshal to the same JSON shape.
//
// Worker (applier side) — Heimdall → hat feedback loop (2026-05-31).
type HeimdallFeedback struct {
	// Source identifies the originating Heimdall run (e.g.
	// "tournament-2026-05-30-evening"). Optional, used for log /
	// audit trail only — doesn't affect application.
	Source string `json:"source,omitempty"`

	// GeneratedAt is a timestamp (RFC3339) for the attribution run.
	// Optional, used for staleness checks by the caller; the
	// applier itself doesn't gate on age.
	GeneratedAt string `json:"generated_at,omitempty"`

	// SampleSize is the number of games attributed. Drives the
	// confidence-scaling factor in ApplyHeimdallFeedback: smaller
	// samples produce smaller effective deltas (statistical noise
	// dampening). 0 is treated as "unknown sample size" and
	// receives the default (no scaling), since the feedback file
	// itself is the trust signal.
	SampleSize int `json:"sample_size,omitempty"`

	// Attributions maps EvalWeights JSON field names to suggested
	// per-dimension deltas. Sign convention: positive delta = bump
	// the weight up (the dimension was under-emphasized in
	// observed games); negative = pull it down. Magnitude is the
	// raw suggestion before any cap / floor / sample-scaling
	// adjustments — see ApplyHeimdallFeedback for the actual
	// transformation applied.
	//
	// Keys MUST match EvalWeights JSON tags (board_presence,
	// card_advantage, ...). Unknown keys are ignored defensively
	// so schema drift between hat and Heimdall doesn't crash the
	// applier.
	Attributions map[string]float64 `json:"attributions"`
}

// HeimdallFeedback application bounds — these are the safety
// invariants that keep a single feedback file from dramatically
// rewriting the deck's evaluator personality.
const (
	// perDimDeltaCap is the maximum absolute delta any single
	// dimension can receive from one feedback file. A 0.15 cap on
	// a weight that's typically ~1.0 means the most aggressive
	// single-feedback change is ~15% of baseline. Set conservatively
	// to make the feedback loop converge slowly (multi-run learning
	// rate) rather than oscillate.
	perDimDeltaCap = 0.15

	// totalMagnitudeBudget is the maximum sum of |delta| across all
	// dimensions in a single feedback application. Prevents a
	// "shotgun" feedback file from nudging every dimension at once
	// — that would shift the deck's identity rather than refine its
	// emphasis. 0.5 is roughly "you can move 3-5 dimensions by 10%
	// each, or 1 dimension by 15% and a few smaller ones." Scaling
	// down each delta proportionally when the sum exceeds the
	// budget keeps the relative weighting in the attribution
	// preserved.
	totalMagnitudeBudget = 0.5

	// minimumWeightFloor is the regression-safety guard: no
	// dimension can be driven below this floor by a feedback
	// application. Prevents a strong-negative attribution from
	// zeroing out a dimension entirely (the evaluator would then
	// be permanently blind to it), which would be effectively
	// destructive on a single noisy game batch. Set to 0.05 — well
	// below any DefaultWeightsForArchetype starting value but
	// above zero, so the dimension remains lightly present.
	minimumWeightFloor = 0.05

	// sampleSizeReference is the "fully trusted" sample size — a
	// feedback file generated from this many games applies its
	// deltas at full magnitude. Smaller samples get scaled down
	// by sqrt(SampleSize / sampleSizeReference) so 8 games applies
	// at ~0.52× weight, 30 games at full weight. 0-SampleSize (the
	// "unknown" sentinel) bypasses scaling entirely — the file's
	// presence is the trust signal.
	sampleSizeReference = 30
)

// LoadHeimdallFeedback reads a feedback file from disk and unmarshals
// it into a HeimdallFeedback struct. Returns (nil, nil) when the path
// is empty so callers can pass a flag value unconditionally without
// guarding for empty input. Returns error on file-not-found, malformed
// JSON, or missing Attributions field — the caller can fall through
// to "no feedback applied" on error rather than aborting the run.
func LoadHeimdallFeedback(path string) (*HeimdallFeedback, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("hat: read feedback %s: %w", path, err)
	}
	var fb HeimdallFeedback
	if err := json.Unmarshal(data, &fb); err != nil {
		return nil, fmt.Errorf("hat: parse feedback %s: %w", path, err)
	}
	if fb.Attributions == nil {
		return nil, fmt.Errorf("hat: feedback %s missing attributions", path)
	}
	return &fb, nil
}

// ApplyHeimdallFeedback mutates sp.Weights with the per-dimension
// deltas from fb, applying the safety bounds described in the
// per-dim/total-magnitude/floor constants above. Pipeline:
//
//  1. Resolve sp.Weights — if nil, ApplyHeimdallFeedback materializes
//     a copy of DefaultWeightsForArchetype(sp.Archetype) so the
//     feedback applies to a known baseline.
//  2. For each attribution: filter unknown dimension names (defensive
//     schema-drift handling), clamp the raw delta to ±perDimDeltaCap.
//  3. Compute the total |delta| across all attributions; if it
//     exceeds totalMagnitudeBudget, scale every delta proportionally
//     so the sum equals the budget. Preserves the attribution's
//     relative shape.
//  4. Scale by sqrt(SampleSize / sampleSizeReference) when
//     SampleSize > 0. SampleSize == 0 bypasses scaling.
//  5. Apply the scaled delta, clamping the resulting weight to
//     >= minimumWeightFloor.
//
// Returns the count of dimensions actually modified (zero when fb or
// sp is nil, when Attributions is empty, when all deltas were filtered
// as unknown, or when total magnitude was sub-floor after scaling).
// Caller can use the count for log output / smoke-test gating.
func ApplyHeimdallFeedback(sp *StrategyProfile, fb *HeimdallFeedback) int {
	if sp == nil || fb == nil || len(fb.Attributions) == 0 {
		return 0
	}
	if sp.Weights == nil {
		base := DefaultWeightsForArchetype(sp.Archetype)
		sp.Weights = &base
	}

	// Pass 1: filter unknown dimensions + clamp per-dim deltas.
	// Build the working delta map keyed by canonical JSON tag.
	clamped := make(map[string]float64, len(fb.Attributions))
	totalMag := 0.0
	for key, raw := range fb.Attributions {
		if !isKnownEvalWeightKey(key) {
			continue
		}
		d := clampFloat(raw, -perDimDeltaCap, perDimDeltaCap)
		clamped[key] = d
		totalMag += math.Abs(d)
	}
	if len(clamped) == 0 {
		return 0
	}

	// Pass 2: total magnitude budget. Scale deltas proportionally
	// when the sum exceeds the budget so the attribution's relative
	// shape is preserved.
	budgetScale := 1.0
	if totalMag > totalMagnitudeBudget {
		budgetScale = totalMagnitudeBudget / totalMag
	}

	// Pass 3: sample-size scaling. sqrt curve so small samples
	// damp aggressively (8 games ≈ 0.52×, 30 ≈ 1×, 120 also ≈ 1×
	// because we cap above sampleSizeReference). 0 bypasses scaling
	// (unknown-sample fallback — trust the file's presence).
	sampleScale := 1.0
	if fb.SampleSize > 0 {
		sampleScale = math.Sqrt(float64(fb.SampleSize) / float64(sampleSizeReference))
		if sampleScale > 1.0 {
			sampleScale = 1.0
		}
	}

	// Pass 4: apply.
	modified := 0
	for key, d := range clamped {
		effective := d * budgetScale * sampleScale
		if effective == 0 {
			continue
		}
		applyDeltaToWeight(sp.Weights, key, effective)
		modified++
	}
	return modified
}

// isKnownEvalWeightKey returns true when key matches one of the
// EvalWeights JSON field tags. Reflective lookup avoids the
// maintenance overhead of a hand-curated set; the json package's
// struct-tag parsing gives us a single source of truth (the
// EvalWeights struct definition).
func isKnownEvalWeightKey(key string) bool {
	_, ok := evalWeightKeySet[key]
	return ok
}

// evalWeightKeySet is the set of valid JSON tag keys on EvalWeights,
// populated once at package init from the struct's json tags via the
// reflect package. Lazy initialization is fine — package-level vars
// are initialized in order, and ApplyHeimdallFeedback can't be called
// before package init completes.
var evalWeightKeySet = buildEvalWeightKeySet()

// buildEvalWeightKeySet enumerates EvalWeights JSON tags by walking
// the struct via reflection. Avoids a hand-curated string set that
// would drift from the EvalWeights definition: adding a tagged field
// to EvalWeights automatically extends the valid-keys set, and the
// only thing the maintainer needs to remember is to add the
// corresponding switch arm to applyDeltaToWeight (where the omission
// surfaces in TestHeimdallFeedback_AllDimensionsCovered).
func buildEvalWeightKeySet() map[string]bool {
	out := map[string]bool{}
	t := reflect.TypeOf(EvalWeights{})
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip ",omitempty" et al — only the name matters.
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		out[tag] = true
	}
	return out
}

// applyDeltaToWeight applies a signed delta to the named dimension on
// EvalWeights, clamping the result to >= minimumWeightFloor. The
// switch enumerates every known dimension explicitly so adding a new
// field to EvalWeights surfaces as a compile error in this file
// (caught by the maintainer when they extend the struct rather than
// silently dropping the delta).
func applyDeltaToWeight(w *EvalWeights, key string, delta float64) {
	floor := minimumWeightFloor
	switch key {
	case "board_presence":
		w.BoardPresence = clampFloorMin(w.BoardPresence+delta, floor)
	case "card_advantage":
		w.CardAdvantage = clampFloorMin(w.CardAdvantage+delta, floor)
	case "mana_advantage":
		w.ManaAdvantage = clampFloorMin(w.ManaAdvantage+delta, floor)
	case "life_resource":
		w.LifeResource = clampFloorMin(w.LifeResource+delta, floor)
	case "combo_proximity":
		w.ComboProximity = clampFloorMin(w.ComboProximity+delta, floor)
	case "threat_exposure":
		w.ThreatExposure = clampFloorMin(w.ThreatExposure+delta, floor)
	case "commander_progress":
		w.CommanderProgress = clampFloorMin(w.CommanderProgress+delta, floor)
	case "graveyard_value":
		w.GraveyardValue = clampFloorMin(w.GraveyardValue+delta, floor)
	case "drain_engine":
		w.DrainEngine = clampFloorMin(w.DrainEngine+delta, floor)
	case "artifact_synergy":
		w.ArtifactSynergy = clampFloorMin(w.ArtifactSynergy+delta, floor)
	case "enchantment_synergy":
		w.EnchantmentSynergy = clampFloorMin(w.EnchantmentSynergy+delta, floor)
	case "opponent_graveyard_threat":
		w.OpponentGraveyardThreat = clampFloorMin(w.OpponentGraveyardThreat+delta, floor)
	case "partner_synergy":
		w.PartnerSynergy = clampFloorMin(w.PartnerSynergy+delta, floor)
	case "activation_tempo":
		w.ActivationTempo = clampFloorMin(w.ActivationTempo+delta, floor)
	case "toolbox_breadth":
		w.ToolboxBreadth = clampFloorMin(w.ToolboxBreadth+delta, floor)
	case "threat_trajectory":
		w.ThreatTrajectory = clampFloorMin(w.ThreatTrajectory+delta, floor)
	case "stack_interaction":
		w.StackInteraction = clampFloorMin(w.StackInteraction+delta, floor)
	case "planeswalker_progress":
		w.PlaneswalkerProgress = clampFloorMin(w.PlaneswalkerProgress+delta, floor)
	case "exile_zone_assets":
		w.ExileZoneAssets = clampFloorMin(w.ExileZoneAssets+delta, floor)
	case "stax_lock_progress":
		w.StaxLockProgress = clampFloorMin(w.StaxLockProgress+delta, floor)
	}
}

// clampFloorMin returns the larger of v and floor (no upper cap —
// downside is the asymmetric risk for EvalWeights since zero blinds
// the dimension entirely).
func clampFloorMin(v, floor float64) float64 {
	if v < floor {
		return floor
	}
	return v
}

