package hat

import (
	"testing"
)

// voltron_defensive_r5_test.go — R60 round 5 retune of Voltron's
// defensive dimensions. PR #251's 5-pod × 500g diversity gauntlet
// measured Wyleth/Voltron at 2.8% mean winrate (σ=0.15) — uniformly
// destroyed across every pod composition. Identical-hat self-play
// over-converges on punishing single-creature plans because every
// opponent makes the same removal decision.
//
// Round 5 pushes Voltron's defensive dials further:
//   - ThreatExposure 1.4 → 1.8 (commander survival = the winrate)
//   - StackInteraction 0.8 → 1.2 (protection spells must actually fire)

func TestVoltronWeights_R5_ThreatExposureAnchorRaised(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeVoltron)
	if w.ThreatExposure < 1.8 {
		t.Errorf("Voltron ThreatExposure should be ≥ 1.8 (R5 self-play tune); got %.2f",
			w.ThreatExposure)
	}
}

func TestVoltronWeights_R5_StackInteractionRaised(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeVoltron)
	if w.StackInteraction < 1.2 {
		t.Errorf("Voltron StackInteraction should be ≥ 1.2 (R5 self-play tune); got %.2f",
			w.StackInteraction)
	}
}

// ThreatExposure now beats Control's (1.2) AND Reanimator's (1.2) —
// the three "single-creature plan" archetypes are now ordered:
// Voltron (1.8) > Reanimator (1.2) > Control (1.2). Voltron's
// dependence is structurally tighter than Reanimator (which can chain
// recursion) or Control (which has counterspells, removal, etc.).
func TestVoltronWeights_R5_ThreatExposureHighestOfDefensiveTier(t *testing.T) {
	v := DefaultWeightsForArchetype(ArchetypeVoltron)
	r := DefaultWeightsForArchetype(ArchetypeReanimator)
	c := DefaultWeightsForArchetype(ArchetypeControl)
	if v.ThreatExposure <= r.ThreatExposure {
		t.Errorf("Voltron ThreatExposure (%.2f) should exceed Reanimator (%.2f) — Voltron has even less plan B",
			v.ThreatExposure, r.ThreatExposure)
	}
	if v.ThreatExposure <= c.ThreatExposure {
		t.Errorf("Voltron ThreatExposure (%.2f) should exceed Control (%.2f) — Control has spell-based redundancy",
			v.ThreatExposure, c.ThreatExposure)
	}
}

// Sanity: CommanderProgress remains highest. The R5 tune raised
// ThreatExposure but must NOT have overtaken the signature dimension —
// CommanderProgress=2.0 still has to anchor the profile.
func TestVoltronWeights_R5_CommanderProgressStillHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeVoltron)
	a := w.AsArray()
	idxCommander := 6 // CommanderProgress slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxCommander {
		t.Fatalf("Voltron's highest weight must remain CommanderProgress (idx %d), got idx %d val %.2f",
			idxCommander, bestIdx, best)
	}
}

// Sanity: the R2-era ArtifactSynergy / EnchantmentSynergy tuning must
// not be disturbed by the R5 changes.
func TestVoltronWeights_R5_R2DimensionsIntact(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeVoltron)
	if w.ArtifactSynergy < 1.0 {
		t.Errorf("Voltron ArtifactSynergy R2 floor (1.0) regressed; got %.2f", w.ArtifactSynergy)
	}
	if w.EnchantmentSynergy < 0.8 {
		t.Errorf("Voltron EnchantmentSynergy R2 floor (0.8) regressed; got %.2f", w.EnchantmentSynergy)
	}
}
