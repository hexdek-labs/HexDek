package hat

import (
	"fmt"
	"sync"

	"github.com/hexdek/hexdek/internal/analytics"
)

// ---------------------------------------------------------------------------
// Heimdall → hat feedback APPLY path — FOUNDATIONAL INTERFACE (R60)
//
// Counterpart to internal/analytics/heimdall_feedback.go (the contract +
// serialization). This file owns the consumer side: a global active
// feedback overlay that DefaultWeightsForArchetype consults to produce
// post-overlay weights for the next gauntlet.
//
// Concurrency model mirrors LegacyMidrangeOnly: SetActiveFeedback should
// be called ONCE at gauntlet startup (before games begin); concurrent
// flips during a tournament are not supported (the per-archetype delta
// lookup is read-only after SetActive but the global itself isn't
// reader-locked). Tests use Clear to reset state between cases.
//
// Apply policies (ApplyPolicy enum) let callers tune how aggressively
// the overlay is honored: Direct (delta added as-is), ConfidenceWeighted
// (delta scaled by per-dimension Confidence), AdvisoryOnly (deltas
// LOADED but ignored, useful for dry-run reports), Skip (no overlay
// applied even if SetActive was called — useful for A/B compare).
//
// Safety bounds: every applied delta runs through analytics.ClampDelta
// AND the final per-dimension weight is floored at 0. The clamp on the
// producer side prevents a healthy producer from emitting >2.0; the
// clamp here prevents a hand-edited / corrupt JSON from doing it. The
// floor prevents a negative weight from inverting MCTS dimension sign.
// ---------------------------------------------------------------------------

// ApplyPolicy controls how SetActiveFeedback's overlay is applied to
// per-archetype weights.
type ApplyPolicy int

const (
	// PolicyDirect: each delta is added directly to the base weight,
	// post-clamp. Default policy.
	PolicyDirect ApplyPolicy = iota
	// PolicyConfidenceWeighted: each delta is multiplied by its
	// per-dimension Confidence ([0,1]) before being added. Low-
	// confidence deltas effectively no-op.
	PolicyConfidenceWeighted
	// PolicyAdvisoryOnly: SetActive succeeds and the feedback is
	// retrievable via ActiveFeedback, but DefaultWeightsForArchetype
	// returns un-overlaid weights. Useful for dry-run reports + the
	// MinSampleSize-too-small case.
	PolicyAdvisoryOnly
	// PolicySkip: SetActive is a no-op (Clear without the side-effect).
	// Useful for A/B tournament splits where one half runs with
	// feedback and one half runs without.
	PolicySkip
)

// activeFeedback + activePolicy + activeMu are the process-wide overlay
// state. Set once at gauntlet startup; never reader-locked at dispatch
// time so the per-archetype lookup path stays as cheap as a map read.
var (
	activeFeedback *analytics.HeimdallFeedback
	activePolicy   = PolicyDirect
	// activeByArchetype is the pre-flattened lookup map populated by
	// SetActiveFeedback. Keyed on archetype name; value is the map of
	// dimension-name → effective delta (already policy-adjusted and
	// clamped). Avoids re-scanning the Archetypes slice on every
	// DefaultWeightsForArchetype call.
	activeByArchetype map[string]map[string]float64
	activeMu          sync.RWMutex
)

// SetActiveFeedback installs the given feedback as the active overlay
// for the next gauntlet. Pass nil to clear. Returns a non-nil error
// only on invalid input; a feedback with SampleSize below
// analytics.MinSampleSize is accepted but downgraded to
// PolicyAdvisoryOnly (loaded but not applied).
//
// MUST be called before the first NewEvaluator of the gauntlet. Calling
// mid-gauntlet is safe in the data-race sense (RWMutex-protected) but
// produces inconsistent per-deck weights across games in the run.
func SetActiveFeedback(fb *analytics.HeimdallFeedback, policy ApplyPolicy) error {
	activeMu.Lock()
	defer activeMu.Unlock()

	if fb == nil {
		activeFeedback = nil
		activeByArchetype = nil
		activePolicy = policy
		return nil
	}
	if fb.SchemaVersion != analytics.HeimdallFeedbackSchemaVersion {
		return fmt.Errorf("hat.SetActiveFeedback: schema version %d unsupported (want %d)",
			fb.SchemaVersion, analytics.HeimdallFeedbackSchemaVersion)
	}
	activeFeedback = fb
	activePolicy = policy

	// Pre-flatten so DefaultWeightsForArchetype is a 2-map-lookup hot
	// path instead of a per-call linear scan over fb.Archetypes.
	flat := make(map[string]map[string]float64, len(fb.Archetypes))
	for _, af := range fb.Archetypes {
		// Per-archetype sample-size downgrade: archetypes with
		// SampleSize < MinSampleSize ship the deltas but the apply
		// path doesn't consume them. Mirrors PolicyAdvisoryOnly at
		// per-archetype granularity.
		if af.SampleSize < analytics.MinSampleSize {
			continue
		}
		if policy == PolicyAdvisoryOnly || policy == PolicySkip {
			continue
		}
		dimMap := make(map[string]float64, len(af.Deltas))
		for _, d := range af.Deltas {
			eff := analytics.ClampDelta(d.Delta)
			if policy == PolicyConfidenceWeighted {
				// Default confidence of 0 means "untrusted" — treat as
				// 1.0 only when explicitly populated. Pre-clamp the
				// confidence to [0,1] to absorb hand-edit drift.
				conf := d.Confidence
				if conf < 0 {
					conf = 0
				} else if conf > 1 {
					conf = 1
				}
				eff = analytics.ClampDelta(eff * conf)
			}
			dimMap[d.Dimension] = eff
		}
		if len(dimMap) > 0 {
			flat[af.Archetype] = dimMap
		}
	}
	activeByArchetype = flat
	return nil
}

// ClearActiveFeedback removes the overlay. Equivalent to SetActiveFeedback(nil, PolicyDirect).
func ClearActiveFeedback() {
	_ = SetActiveFeedback(nil, PolicyDirect)
}

// ActiveFeedback returns the currently-installed feedback (nil when
// none). Useful for reports / dry-run inspection; should NOT be used
// for hot-path dispatch (use DefaultWeightsForArchetype which already
// honors the overlay).
func ActiveFeedback() *analytics.HeimdallFeedback {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activeFeedback
}

// ActiveApplyPolicy returns the policy installed alongside the active
// feedback. Returns PolicyDirect when no feedback is installed.
func ActiveApplyPolicy() ApplyPolicy {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activePolicy
}

// applyFeedbackOverlay applies any installed Heimdall feedback overlay
// to the base weights for the given archetype. Returns the input
// unchanged when no feedback is active or no per-archetype deltas
// exist. Called from DefaultWeightsForArchetype.
//
// This is a pure function over the per-call snapshot of
// activeByArchetype — concurrent SetActiveFeedback during dispatch is
// safe but may apply the new overlay to the next call rather than the
// in-flight one.
func applyFeedbackOverlay(archetype string, base EvalWeights) EvalWeights {
	activeMu.RLock()
	dims := activeByArchetype[archetype]
	activeMu.RUnlock()
	if len(dims) == 0 {
		return base
	}
	// JSON-tag dispatch: mirror the EvalWeights struct's `json:"..."`
	// tags so additive dimension growth doesn't require touching this
	// switch. Keep aligned with EvalWeights — adding a new dimension
	// there means adding a new case here.
	for dim, delta := range dims {
		switch dim {
		case "board_presence":
			base.BoardPresence = floorAtZero(base.BoardPresence + delta)
		case "card_advantage":
			base.CardAdvantage = floorAtZero(base.CardAdvantage + delta)
		case "mana_advantage":
			base.ManaAdvantage = floorAtZero(base.ManaAdvantage + delta)
		case "life_resource":
			base.LifeResource = floorAtZero(base.LifeResource + delta)
		case "combo_proximity":
			base.ComboProximity = floorAtZero(base.ComboProximity + delta)
		case "threat_exposure":
			base.ThreatExposure = floorAtZero(base.ThreatExposure + delta)
		case "commander_progress":
			base.CommanderProgress = floorAtZero(base.CommanderProgress + delta)
		case "graveyard_value":
			base.GraveyardValue = floorAtZero(base.GraveyardValue + delta)
		case "drain_engine":
			base.DrainEngine = floorAtZero(base.DrainEngine + delta)
		case "artifact_synergy":
			base.ArtifactSynergy = floorAtZero(base.ArtifactSynergy + delta)
		case "enchantment_synergy":
			base.EnchantmentSynergy = floorAtZero(base.EnchantmentSynergy + delta)
		case "opponent_graveyard_threat":
			base.OpponentGraveyardThreat = floorAtZero(base.OpponentGraveyardThreat + delta)
		case "partner_synergy":
			base.PartnerSynergy = floorAtZero(base.PartnerSynergy + delta)
		case "activation_tempo":
			base.ActivationTempo = floorAtZero(base.ActivationTempo + delta)
		case "toolbox_breadth":
			base.ToolboxBreadth = floorAtZero(base.ToolboxBreadth + delta)
		case "threat_trajectory":
			base.ThreatTrajectory = floorAtZero(base.ThreatTrajectory + delta)
		case "stack_interaction":
			base.StackInteraction = floorAtZero(base.StackInteraction + delta)
		case "planeswalker_progress":
			base.PlaneswalkerProgress = floorAtZero(base.PlaneswalkerProgress + delta)
		case "exile_zone_assets":
			base.ExileZoneAssets = floorAtZero(base.ExileZoneAssets + delta)
		case "stax_lock_progress":
			base.StaxLockProgress = floorAtZero(base.StaxLockProgress + delta)
			// Unknown dimensions silently no-op so a future producer can
			// emit new dimensions without breaking older consumers.
		}
	}
	return base
}

// floorAtZero guards against the overlay pushing a weight below 0. A
// negative weight inverts MCTS sign on that dimension and produces
// catastrophic AI play (it's safer to fully zero a dimension than to
// flip it). Producers shouldn't emit deltas that exceed the base weight
// in magnitude, but a hand-edited JSON could.
func floorAtZero(v float64) float64 {
	if v < 0 {
		return 0
	}
	return v
}
