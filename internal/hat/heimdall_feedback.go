package hat

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"time"
)

// realTimeNow is the production timestamp source used by MarkFeedbackRejected
// when timeNow hasn't been overridden for tests. Aliased through a
// package-level var so the injection-point pattern stays one line.
func realTimeNow() time.Time { return time.Now().UTC() }

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

// ---------------------------------------------------------------------
// Rollback safety net (2026-05-31)
//
// The applier above can nudge the deck's eval weights based on
// Heimdall attribution, but the attribution itself can be noisy or
// misleading (small sample, biased opponent pool, regression in the
// Heimdall analysis itself). If applying a feedback file actively
// HARMS the deck's next gauntlet, we want a way to undo it
// automatically rather than waiting for a human to notice the win-rate
// drop and roll back by hand.
//
// The pipeline:
//
//   1. Before apply: caller captures FeedbackSnapshot via
//      ApplyHeimdallFeedbackWithSnapshot. Snapshot holds a value-copy
//      of the pre-mutation Weights plus audit metadata.
//
//   2. After apply: caller runs the gauntlet, computes the observed
//      win rate.
//
//   3. Decision: caller passes baselineWinRate (pre-feedback) and
//      currentWinRate (post-feedback) to ShouldRollback with the
//      configured threshold (typically 0.05 = 5pp drop). Returns
//      true when current < baseline - threshold.
//
//   4. Rollback: caller invokes RollbackFromSnapshot to restore
//      Weights, then MarkFeedbackRejected to write a sentinel
//      sidecar (<feedbackPath>.rejected) so the NEXT --apply-feedback
//      invocation with the same path detects the rejection and
//      skips application (via IsFeedbackRejected).
//
// The sidecar persists the rejection across runs — without it, the
// next tournament would re-apply the same bad feedback and the cycle
// would repeat. Sidecars are JSON for human-readable audit
// (timestamp, baseline/current/threshold).

// FeedbackSnapshot captures the state of a StrategyProfile's eval
// weights BEFORE a HeimdallFeedback application, plus audit metadata
// (source file path, dimensions modified) for the rollback decision.
// Created by ApplyHeimdallFeedbackWithSnapshot; consumed by
// RollbackFromSnapshot. Always a value-copy — restoring the snapshot
// gives the caller a byte-for-byte restoration of the pre-application
// state.
type FeedbackSnapshot struct {
	// PreApplicationWeights is a value-copy of sp.Weights taken
	// immediately before the applier ran. RollbackFromSnapshot
	// writes this back to sp.Weights to undo the mutation. Stored
	// by value (not pointer) so future mutations to sp.Weights
	// don't corrupt the snapshot.
	PreApplicationWeights EvalWeights

	// FeedbackSource mirrors HeimdallFeedback.Source for audit /
	// log output. Empty when the feedback file didn't supply one.
	FeedbackSource string

	// DimensionsModified is the count of dimensions actually
	// changed by the apply call (same value the original
	// ApplyHeimdallFeedback returned). 0 indicates a no-op apply
	// — snapshot is still valid but rollback would be a no-op.
	DimensionsModified int
}

// ApplyHeimdallFeedbackWithSnapshot is the snapshot-capable variant
// of ApplyHeimdallFeedback for callers that want rollback capability.
// Captures a value-copy of sp.Weights BEFORE the apply, runs the
// existing apply pipeline, and returns the modification count plus
// the snapshot. Snapshot is nil only when sp itself is nil (the
// caller can't meaningfully roll back anything without a profile).
//
// Snapshot materializes the archetype defaults when sp.Weights was
// nil at call time, mirroring ApplyHeimdallFeedback's own
// materialization — so rollback restores to the materialized
// baseline, not to a nil pointer.
func ApplyHeimdallFeedbackWithSnapshot(sp *StrategyProfile, fb *HeimdallFeedback) (int, *FeedbackSnapshot) {
	if sp == nil {
		return 0, nil
	}
	// Pre-materialize so the snapshot captures the same baseline
	// that the applier would have used. Mirrors the applier's own
	// nil-Weights handling.
	if sp.Weights == nil {
		base := DefaultWeightsForArchetype(sp.Archetype)
		sp.Weights = &base
	}
	snap := &FeedbackSnapshot{
		PreApplicationWeights: *sp.Weights, // value-copy
	}
	if fb != nil {
		snap.FeedbackSource = fb.Source
	}
	n := ApplyHeimdallFeedback(sp, fb)
	snap.DimensionsModified = n
	return n, snap
}

// RollbackFromSnapshot restores sp.Weights to the pre-application
// state captured in snapshot. No-op when either sp or snapshot is
// nil. Idempotent: rolling back to the same snapshot twice produces
// the same result as rolling back once.
//
// Materializes sp.Weights as a fresh pointer (rather than mutating
// in place) so any other references to the old *EvalWeights see
// neither the post-feedback nor the rolled-back state mid-rollback —
// the rollback is atomic from the consumer's perspective.
func RollbackFromSnapshot(sp *StrategyProfile, snapshot *FeedbackSnapshot) {
	if sp == nil || snapshot == nil {
		return
	}
	restored := snapshot.PreApplicationWeights // value-copy out
	sp.Weights = &restored
}

// ShouldRollback returns true when the observed post-feedback win
// rate dropped more than `threshold` below the pre-feedback baseline.
// Rates and threshold are fractions in [0, 1] (so 0.05 = 5 percentage
// points). Returns false (do not roll back) on degenerate inputs:
//
//   - baselineWinRate <= 0 (no baseline supplied — can't compute drop)
//   - threshold <= 0 (rollback gate disabled by configuration)
//   - currentWinRate >= baselineWinRate - threshold (no regression
//     significant enough to revert)
//
// The asymmetric "<= 0 = disabled" semantic for both baseline and
// threshold lets callers wire a single rollback flag pair and have it
// short-circuit cleanly when either side is missing — no need for a
// separate "rollback enabled" boolean.
func ShouldRollback(baselineWinRate, currentWinRate, threshold float64) bool {
	if baselineWinRate <= 0 || threshold <= 0 {
		return false
	}
	drop := baselineWinRate - currentWinRate
	return drop > threshold
}

// rejectionSidecarSuffix is appended to a feedback file path to form
// the rejection-sentinel path: feedback.json → feedback.json.rejected.
// Suffix rather than a separate directory so the marker travels with
// the feedback file across copies / moves.
const rejectionSidecarSuffix = ".rejected"

// FeedbackRejection is the on-disk shape of the rejection sidecar.
// Human-readable JSON so an operator inspecting the file gets the
// full context (which gauntlet, what baseline vs observed rate,
// what threshold tripped). Loaded by IsFeedbackRejectedDetail when
// callers want the rejection record beyond the boolean.
type FeedbackRejection struct {
	FeedbackPath    string  `json:"feedback_path"`
	BaselineWinRate float64 `json:"baseline_win_rate"`
	CurrentWinRate  float64 `json:"current_win_rate"`
	Threshold       float64 `json:"threshold"`
	RejectedAt      string  `json:"rejected_at"`
	Note            string  `json:"note,omitempty"`
}

// MarkFeedbackRejected writes a FeedbackRejection sidecar at
// <feedbackPath>.rejected so the NEXT --apply-feedback invocation
// with the same path can detect the rejection and skip application
// (via IsFeedbackRejected). Idempotent: calling twice overwrites
// with the latest rejection metadata.
//
// Caller-supplied note is optional human-readable context (e.g.
// "rollback after 100-game gauntlet on dev/foo-bar"). Empty path is
// a no-op — defensive for the "rollback decided but feedback was
// loaded from an empty-path fallback" case.
func MarkFeedbackRejected(feedbackPath string, baselineWinRate, currentWinRate, threshold float64, note string) error {
	if feedbackPath == "" {
		return nil
	}
	rej := FeedbackRejection{
		FeedbackPath:    feedbackPath,
		BaselineWinRate: baselineWinRate,
		CurrentWinRate:  currentWinRate,
		Threshold:       threshold,
		RejectedAt:      timeNow().Format("2006-01-02T15:04:05Z07:00"),
		Note:            note,
	}
	data, err := json.MarshalIndent(rej, "", "  ")
	if err != nil {
		return fmt.Errorf("hat: marshal rejection sidecar: %w", err)
	}
	sidecar := feedbackPath + rejectionSidecarSuffix
	if err := os.WriteFile(sidecar, data, 0o644); err != nil {
		return fmt.Errorf("hat: write rejection sidecar %s: %w", sidecar, err)
	}
	return nil
}

// IsFeedbackRejected returns true when a rejection sidecar exists at
// <feedbackPath>.rejected. Cheap stat-only check; callers that need
// the rejection metadata (timestamp, baseline, etc.) should call
// LoadFeedbackRejection instead. Empty path returns false (defensive).
func IsFeedbackRejected(feedbackPath string) bool {
	if feedbackPath == "" {
		return false
	}
	_, err := os.Stat(feedbackPath + rejectionSidecarSuffix)
	return err == nil
}

// LoadFeedbackRejection returns the rejection sidecar record for the
// given feedback path, or (nil, nil) when no sidecar exists. Returns
// an error only on read / parse failure of an existing sidecar.
func LoadFeedbackRejection(feedbackPath string) (*FeedbackRejection, error) {
	if feedbackPath == "" {
		return nil, nil
	}
	sidecar := feedbackPath + rejectionSidecarSuffix
	data, err := os.ReadFile(sidecar)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("hat: read rejection sidecar %s: %w", sidecar, err)
	}
	var rej FeedbackRejection
	if err := json.Unmarshal(data, &rej); err != nil {
		return nil, fmt.Errorf("hat: parse rejection sidecar %s: %w", sidecar, err)
	}
	return &rej, nil
}

// ClearFeedbackRejection deletes the rejection sidecar at
// <feedbackPath>.rejected, re-enabling application of the feedback
// file on subsequent runs. Used by operators who want to retry a
// previously-rejected feedback file (e.g. after re-running Heimdall
// with a larger sample size). No-op when no sidecar exists.
func ClearFeedbackRejection(feedbackPath string) error {
	if feedbackPath == "" {
		return nil
	}
	sidecar := feedbackPath + rejectionSidecarSuffix
	if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("hat: remove rejection sidecar %s: %w", sidecar, err)
	}
	return nil
}

// timeNow is a package-level injection point for time.Now() so tests
// can pin a deterministic timestamp in MarkFeedbackRejected output.
// Defaults to the real time.Now. Tests swap in a fixed-time function
// then restore.
var timeNow = realTimeNow
