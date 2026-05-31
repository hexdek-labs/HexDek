package hat

import (
	"math"
	"testing"
)

// r60-archetype-fingerprint (PR #943 followup): pin distinctness floors
// for every customized archetype profile so future tunings cannot
// regress an archetype back toward the midrange baseline. The metric
// is L1 distance from the Midrange profile across all 20 EvalWeights
// dimensions — easy to compute, monotonic in "how unique does this
// archetype look to MCTS."
//
// The brief came from PR #923's self-play null result + PR #943's
// amplification ablation: the dispatch works but archetypes whose
// weights cluster near midrange produce no per-deck signal at n=500.
// Voltron at L1=7.6 produces signal; Combo at L1=4.5 (pre-amp) did
// not. The floor of 5.0 below is calibrated so an archetype "as
// distinct as pre-amp Combo" would now fail — forcing future
// regressions to either lower the floor (with explicit acknowledgment)
// or restore the differentiation.

// l1FromMidrange computes the L1 distance between the given archetype's
// weight profile and Midrange across all 20 dimensions. Returns -1 when
// archetypeWeights is missing the entry.
func l1FromMidrange(archetype string) float64 {
	a, ok := archetypeWeights[archetype]
	if !ok {
		return -1
	}
	m := archetypeWeights[ArchetypeMidrange]
	aa := a.AsArray()
	mm := m.AsArray()
	sum := 0.0
	for i := range aa {
		sum += math.Abs(aa[i] - mm[i])
	}
	return sum
}

// TestArchetypeFingerprint_AllCustomizedAreDistinct pins the headline
// invariant: every archetype that's been explicitly customized has
// L1 ≥ 5.0 from Midrange. The pre-amp Combo profile at L1 = 4.5 would
// have failed this; the post-amp 5.5+ (Toolbox 0.6→1.0, StackInt
// 0.6→1.0) passes.
//
// The 5.0 floor is calibrated to be just above the pre-amp Combo
// number so future regressions can't put any archetype below the
// known "produces no self-play signal" threshold without explicitly
// lowering this floor.
func TestArchetypeFingerprint_AllCustomizedAreDistinct(t *testing.T) {
	const minDistinctness = 5.0
	customized := []string{
		ArchetypeAggro,
		ArchetypeCombo,
		ArchetypeControl,
		ArchetypeStax,
		ArchetypeStorm,
		ArchetypeVoltron,
		ArchetypeReanimator,
	}
	for _, a := range customized {
		l1 := l1FromMidrange(a)
		if l1 < 0 {
			t.Errorf("archetype %q not in archetypeWeights map", a)
			continue
		}
		if l1 < minDistinctness {
			t.Errorf("archetype %q L1 from Midrange = %.2f, want >= %.2f (PR #923/#943 self-play null floor)",
				a, l1, minDistinctness)
		}
	}
}

// TestArchetypeFingerprint_ComboAmpLanded pins the specific amp values
// this PR ships. Defends against an accidental revert that would lower
// L1 distinctness back below the 5.0 floor.
func TestArchetypeFingerprint_ComboAmpLanded(t *testing.T) {
	w := archetypeWeights[ArchetypeCombo]
	if w.ToolboxBreadth != 1.0 {
		t.Errorf("Combo ToolboxBreadth = %.2f, want 1.0 (r60-amp from 0.6)", w.ToolboxBreadth)
	}
	if w.StackInteraction != 1.0 {
		t.Errorf("Combo StackInteraction = %.2f, want 1.0 (r60-amp from 0.6)", w.StackInteraction)
	}
	if w.ComboProximity != 2.0 {
		t.Errorf("Combo ComboProximity = %.2f, want 2.0 (peak, unchanged)", w.ComboProximity)
	}
}

// TestArchetypeFingerprint_ReanimatorAmpLanded pins Reanimator's
// secondary-dim amplification.
func TestArchetypeFingerprint_ReanimatorAmpLanded(t *testing.T) {
	w := archetypeWeights[ArchetypeReanimator]
	if w.OpponentGraveyardThreat != 1.1 {
		t.Errorf("Reanimator OpponentGraveyardThreat = %.2f, want 1.1 (r60-amp from 0.8)", w.OpponentGraveyardThreat)
	}
	if w.ActivationTempo != 0.9 {
		t.Errorf("Reanimator ActivationTempo = %.2f, want 0.9 (r60-amp from 0.6)", w.ActivationTempo)
	}
	if w.GraveyardValue != 1.8 {
		t.Errorf("Reanimator GraveyardValue = %.2f, want 1.8 (peak, unchanged)", w.GraveyardValue)
	}
}

// TestArchetypeFingerprint_ControlAmpLanded pins Control's secondary-
// dim amplification.
func TestArchetypeFingerprint_ControlAmpLanded(t *testing.T) {
	w := archetypeWeights[ArchetypeControl]
	if w.OpponentGraveyardThreat != 1.3 {
		t.Errorf("Control OpponentGraveyardThreat = %.2f, want 1.3 (r60-amp from 1.0)", w.OpponentGraveyardThreat)
	}
	if w.ThreatTrajectory != 1.1 {
		t.Errorf("Control ThreatTrajectory = %.2f, want 1.1 (r60-amp from 0.8)", w.ThreatTrajectory)
	}
	if w.CardAdvantage != 1.6 {
		t.Errorf("Control CardAdvantage = %.2f, want 1.6 (peak, unchanged)", w.CardAdvantage)
	}
}

// TestArchetypeFingerprint_PairwiseDistinctness pins a stronger
// invariant: every pair of customized archetypes is distinct from
// EACH OTHER, not just from Midrange. Floor is lower (3.5) because
// two adjacent archetypes (Combo vs Spellslinger, Stax vs Control)
// can legitimately share dial profiles for some dimensions.
//
// This catches regressions where amplifying one archetype
// inadvertently collapses it onto a peer (e.g., Combo + Spellslinger
// both pumping CardAdvantage + ComboProximity would converge).
func TestArchetypeFingerprint_PairwiseDistinctness(t *testing.T) {
	const minPairwise = 3.5
	customized := []string{
		ArchetypeAggro,
		ArchetypeCombo,
		ArchetypeControl,
		ArchetypeStax,
		ArchetypeStorm,
		ArchetypeVoltron,
		ArchetypeReanimator,
	}

	pairL1 := func(a, b string) float64 {
		aa := archetypeWeights[a].AsArray()
		bb := archetypeWeights[b].AsArray()
		sum := 0.0
		for i := range aa {
			sum += math.Abs(aa[i] - bb[i])
		}
		return sum
	}

	for i := 0; i < len(customized); i++ {
		for j := i + 1; j < len(customized); j++ {
			a, b := customized[i], customized[j]
			l1 := pairL1(a, b)
			if l1 < minPairwise {
				t.Errorf("pairwise L1(%s, %s) = %.2f, want >= %.2f (archetypes have collapsed onto each other)",
					a, b, l1, minPairwise)
			}
		}
	}
}
