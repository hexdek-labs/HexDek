package analytics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ---------------------------------------------------------------------------
// Heimdall → hat feedback loop — FOUNDATIONAL INTERFACE (R60)
//
// Until this commit, the arrow only went one way: Heimdall observed games
// (analytics.GameAnalysis) and exported insights to humans. Hat's per-
// archetype EvalWeights (internal/hat/eval_weights.go) were hand-tuned by
// engineers reading those reports.
//
// This package adds the typed loopback contract: after a gauntlet,
// Heimdall can compute a HeimdallFeedback delta vector ("decision class X
// was misweighted by Y in archetype Z") and persist it; the hat package
// (internal/hat/heimdall_feedback_apply.go) consumes the same JSON on
// the next gauntlet's startup and overlays the deltas onto the
// archetype-default weights.
//
// SCOPE OF THIS PR (intentional — see dev/heimdall-hat-feedback-loop-r60 spec):
//
//   - SHIPS:  the contract — HeimdallFeedback struct + per-archetype
//             per-dimension delta encoding, JSON serialization with
//             schema version + provenance, loader, save-atomic helper.
//   - SHIPS:  the integration point — hat package consumes the JSON and
//             applies it via the Apply / SetActive global pattern that
//             mirrors LegacyMidrangeOnly.
//   - SHIPS:  bounds + clamp semantics so a bad/adversarial feedback can
//             never push a weight negative or into the deep tens.
//   - STUBS:  ComputeWeightDeltas — the analytics-side computer that
//             turns a slice of GameAnalysis into a HeimdallFeedback.
//             Returns an empty feedback (zero deltas, "stub" provenance)
//             today; dev-4 / dev-13 fill in the body when the actual
//             tuning-loop scoring lands. The contract is committed so
//             those PRs don't need to renegotiate the schema.
//   - SKIPS:  the gauntlet harness change that calls
//             SaveHeimdallFeedback at the end of a run. Belongs in the
//             tournament CLI, not this foundational PR.
//
// Schema version is committed at 1. A breaking change requires a bump
// and a migration shim in LoadHeimdallFeedback; downstream tuning-loop
// PRs should NOT bump it without coordination — they should extend with
// new optional fields (additive evolution).
// ---------------------------------------------------------------------------

// HeimdallFeedbackSchemaVersion is the on-disk format version. Bump only
// for breaking changes; additive fields don't require a bump.
const HeimdallFeedbackSchemaVersion = 1

// DimensionDelta is a per-dimension weight adjustment derived from
// Heimdall's analysis of a gauntlet. Positive values mean "the hat
// underweighted this dimension in this archetype"; negative values mean
// "overweighted". Bounded to ±MaxDimensionDelta at compute time; the
// apply path additionally clamps the final weight to ≥ 0.
//
// Dimension is the EvalWeights field name (board_presence, card_advantage,
// mana_advantage, life_resource, combo_proximity, threat_exposure,
// commander_progress, graveyard_value, drain_engine, artifact_synergy,
// enchantment_synergy, opponent_graveyard_threat, partner_synergy,
// activation_tempo, toolbox_breadth, threat_trajectory, stack_interaction,
// planeswalker_progress, exile_zone_assets, stax_lock_progress). Mirrors
// the JSON struct tags on EvalWeights — the loader does a name-keyed
// dispatch rather than a positional index so additive dimension growth
// stays compatible across versions.
type DimensionDelta struct {
	Dimension string  `json:"dimension"`
	Delta     float64 `json:"delta"`
	// Confidence is the producer's self-rated confidence in the delta in
	// [0, 1]. The apply path optionally scales the delta by Confidence
	// when the consumer's apply-policy is set to ConfidenceWeighted.
	// Stub computers should emit 0.5; production computers emit values
	// derived from observation count + variance.
	Confidence float64 `json:"confidence,omitempty"`
	// Reason is human-readable provenance: what observation drove this
	// delta. e.g. "aggressive_aristocrats_avg_win_turn=8.2, baseline=11
	// → DrainEngine underweighted". Optional; useful for debugging
	// surprise tuning shifts in a gauntlet report.
	Reason string `json:"reason,omitempty"`
}

// ArchetypeFeedback bundles every DimensionDelta for one archetype, plus
// the sample-size context the deltas were computed from.
type ArchetypeFeedback struct {
	Archetype string           `json:"archetype"`
	Deltas    []DimensionDelta `json:"deltas"`
	// SampleSize is the number of gauntlet games the deltas were
	// computed from. The apply path treats SampleSize < MinSampleSize
	// as advisory-only (logged, but not applied) so a 5-game pilot run
	// can't shift the production weights.
	SampleSize int `json:"sample_size"`
	// AvgWinRate of the archetype across the sample. Pure-context; the
	// apply path doesn't read this, but reports do.
	AvgWinRate float64 `json:"avg_win_rate,omitempty"`
}

// HeimdallFeedback is the top-level on-disk feedback document. One per
// gauntlet run. Produced by ComputeWeightDeltas, consumed by the hat
// package's Apply / SetActive path.
type HeimdallFeedback struct {
	SchemaVersion int                 `json:"schema_version"`
	GeneratedAt   string              `json:"generated_at"`
	// Provenance is "stub", "tournament:<name>", "loki:<seed>", etc. —
	// records WHO produced the feedback for debugging adversarial-input
	// or stale-feedback scenarios.
	Provenance    string              `json:"provenance"`
	// GauntletGames is the total game count this feedback was computed
	// from (sum across archetypes minus per-archetype double-counting).
	GauntletGames int                 `json:"gauntlet_games"`
	Archetypes    []ArchetypeFeedback `json:"archetypes"`
}

// Compute-time clamps applied by ComputeWeightDeltas. The apply path
// re-clamps post-overlay to defend against adversarial input from a
// hand-edited feedback file.
const (
	// MaxDimensionDelta bounds a single per-dimension adjustment.
	// ±2.0 lets a single gauntlet shift a dimension by up to one "anchor
	// tier" (most archetype profiles use 0.0 / 0.4 / 1.0 / 1.6 / 2.0 as
	// the rough tier landings) but can't flip a profile end-to-end.
	MaxDimensionDelta = 2.0
	// MinSampleSize is the threshold below which the apply path treats
	// the feedback as advisory-only (logged, not applied). 50 games is
	// the smallest sample where per-archetype win-rate variance is
	// tractable; smaller samples ship the JSON but the loader's
	// SetActive path will skip applying.
	MinSampleSize = 50
)

// ClampDelta enforces MaxDimensionDelta in either direction. Used by
// both producer and consumer paths so a hand-edited feedback can't
// bypass the bound.
func ClampDelta(d float64) float64 {
	if d > MaxDimensionDelta {
		return MaxDimensionDelta
	}
	if d < -MaxDimensionDelta {
		return -MaxDimensionDelta
	}
	return d
}

// HeimdallFeedbackFile is the canonical filename written under the dir
// passed to SaveHeimdallFeedback / LoadHeimdallFeedback. Lives alongside
// data/huginn/learned_interactions.json — same file-discipline pattern.
const HeimdallFeedbackFile = "heimdall_feedback.json"

// SaveHeimdallFeedback atomically writes the feedback to
// <dir>/heimdall_feedback.json. Mkdir-s the dir if needed. Sets
// SchemaVersion and GeneratedAt on the input if either is unset so
// callers don't have to remember.
func SaveHeimdallFeedback(dir string, fb *HeimdallFeedback) error {
	if fb == nil {
		return fmt.Errorf("analytics.SaveHeimdallFeedback: nil feedback")
	}
	if fb.SchemaVersion == 0 {
		fb.SchemaVersion = HeimdallFeedbackSchemaVersion
	}
	if fb.GeneratedAt == "" {
		fb.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("analytics.SaveHeimdallFeedback: mkdir: %w", err)
	}
	data, err := json.MarshalIndent(fb, "", "  ")
	if err != nil {
		return fmt.Errorf("analytics.SaveHeimdallFeedback: marshal: %w", err)
	}
	final := filepath.Join(dir, HeimdallFeedbackFile)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("analytics.SaveHeimdallFeedback: write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("analytics.SaveHeimdallFeedback: rename: %w", err)
	}
	return nil
}

// LoadHeimdallFeedback reads <dir>/heimdall_feedback.json. Returns
// (nil, nil) when the file doesn't exist — the apply path treats "no
// feedback" identically to "empty feedback" so gauntlets can run before
// any feedback has been produced. SchemaVersion mismatch returns an
// error rather than silently degrading; downstream additive evolution
// must not bump the version.
func LoadHeimdallFeedback(dir string) (*HeimdallFeedback, error) {
	path := filepath.Join(dir, HeimdallFeedbackFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("analytics.LoadHeimdallFeedback: read %s: %w", path, err)
	}
	var fb HeimdallFeedback
	if err := json.Unmarshal(data, &fb); err != nil {
		return nil, fmt.Errorf("analytics.LoadHeimdallFeedback: parse %s: %w", path, err)
	}
	if fb.SchemaVersion != HeimdallFeedbackSchemaVersion {
		return nil, fmt.Errorf("analytics.LoadHeimdallFeedback: schema version mismatch in %s: got %d, want %d",
			path, fb.SchemaVersion, HeimdallFeedbackSchemaVersion)
	}
	// Re-clamp on load so a hand-edited file can't bypass bounds.
	for i := range fb.Archetypes {
		for j := range fb.Archetypes[i].Deltas {
			fb.Archetypes[i].Deltas[j].Delta = ClampDelta(fb.Archetypes[i].Deltas[j].Delta)
		}
	}
	return &fb, nil
}

// ComputeWeightDeltas turns a slice of GameAnalysis into a
// HeimdallFeedback. STUB IMPLEMENTATION (R60 foundational): returns an
// empty feedback with provenance="stub" and zero deltas per archetype.
// The signature is committed so downstream tuning-loop PRs (dev-4 /
// dev-13) fill in the body without renegotiating the contract.
//
// Future body sketch: per archetype, bucket games by outcome (won, lost,
// drew, conceded); compute regression of each dimension's contribution
// against win-rate residuals; emit deltas where the regression
// coefficient has p < 0.05 and abs(delta) > some-noise-floor.
func ComputeWeightDeltas(games []*GameAnalysis, provenance string) *HeimdallFeedback {
	fb := &HeimdallFeedback{
		SchemaVersion: HeimdallFeedbackSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Provenance:    provenance,
		GauntletGames: len(games),
		// Archetypes is intentionally empty — stub-marker. Apply path
		// no-ops when Archetypes is empty (mirrors the "no feedback"
		// case), so a stub-produced feedback is safe to ship through
		// the pipeline end-to-end without affecting weights.
	}
	if provenance == "" {
		fb.Provenance = "stub"
	}
	return fb
}
