package hat

import (
	"testing"
)

// eval_weights_control_r60_test.go — Round 4 of the R60 archetype-
// weight audit (companion to PR #179 / #181 / round 3 which retuned
// Storm + Mill, Voltron + Aristocrats, and Reanimator + Spellslinger
// + Tribal). Control specifically had three core dials misaligned
// with how Control wins games:
//   - ManaAdvantage = 0.8 (below midrange's 0.8 baseline) ignored
//     Control's defining tempo edge of holding up instant-speed mana
//   - ThreatExposure = 1.2 under-weighted "correctly rank and
//     neutralize the biggest opp threat" — the core Control skill
//   - BoardPresence = 0.5 was below the midrange neutral of 1.0;
//     Control still needs a finisher / blocker count, just not the
//     aggro+1.5 wide-board pressure

// -----------------------------------------------------------------------------
// Control: CardAdvantage stays highest; ManaAdvantage + ThreatExposure rise;
// BoardPresence comes back up to midrange-neutral.
// -----------------------------------------------------------------------------

func TestControlWeights_CardAdvantageRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeControl)
	a := w.AsArray()
	idxCA := 1 // CardAdvantage slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxCA {
		t.Fatalf("Control's highest weight should be CardAdvantage (idx %d), got idx %d val %.2f",
			idxCA, bestIdx, best)
	}
}

func TestControlWeights_CardAdvantageHigh(t *testing.T) {
	// Control's long-game plan is to outdraw the table. Card advantage
	// should remain high (≥ 1.5) and clearly above midrange.
	w := DefaultWeightsForArchetype(ArchetypeControl)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CardAdvantage < 1.5 {
		t.Errorf("Control CardAdvantage should be ≥ 1.5 (outdraw the table); got %.2f",
			w.CardAdvantage)
	}
	if w.CardAdvantage <= mid.CardAdvantage {
		t.Errorf("Control CardAdvantage (%.2f) should exceed midrange (%.2f)",
			w.CardAdvantage, mid.CardAdvantage)
	}
}

func TestControlWeights_ManaAdvantageHigh(t *testing.T) {
	// Control wins by leaving Counterspell / Swords mana up on the
	// opponent's end step. The pre-tune 0.8 ranked Control's mana
	// weight equal to midrange — actively wrong for a draw-go plan.
	w := DefaultWeightsForArchetype(ArchetypeControl)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ManaAdvantage < 1.2 {
		t.Errorf("Control ManaAdvantage should be ≥ 1.2 (hold-up mana for instants); got %.2f",
			w.ManaAdvantage)
	}
	if w.ManaAdvantage <= mid.ManaAdvantage {
		t.Errorf("Control ManaAdvantage (%.2f) should exceed midrange (%.2f)",
			w.ManaAdvantage, mid.ManaAdvantage)
	}
}

func TestControlWeights_ThreatExposureUp(t *testing.T) {
	// Control's core skill is correctly ranking opponent threats and
	// answering the biggest one. ThreatExposure should be > pre-tune
	// (1.2) and clearly above midrange (0.8). Capped under Voltron's
	// 1.4 to preserve the round-2 invariant — Voltron's single-threat
	// fragility makes it the most ThreatExposure-sensitive archetype.
	w := DefaultWeightsForArchetype(ArchetypeControl)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	voltron := DefaultWeightsForArchetype(ArchetypeVoltron)
	if w.ThreatExposure < 1.3 {
		t.Errorf("Control ThreatExposure should be ≥ 1.3 (rank + neutralize biggest opp threat); got %.2f",
			w.ThreatExposure)
	}
	if w.ThreatExposure <= mid.ThreatExposure {
		t.Errorf("Control ThreatExposure (%.2f) should exceed midrange (%.2f)",
			w.ThreatExposure, mid.ThreatExposure)
	}
	if w.ThreatExposure >= voltron.ThreatExposure {
		t.Errorf("Control ThreatExposure (%.2f) should stay below Voltron (%.2f) per round-2 invariant",
			w.ThreatExposure, voltron.ThreatExposure)
	}
}

func TestControlWeights_BoardPresenceNeutral(t *testing.T) {
	// Control still needs blockers and a finisher count; the pre-tune
	// 0.5 made the evaluator treat any non-empty board as wasted weight.
	// Neutral = midrange baseline (1.0); don't push past it because
	// Control doesn't pressure with width.
	w := DefaultWeightsForArchetype(ArchetypeControl)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	aggro := DefaultWeightsForArchetype(ArchetypeAggro)

	if w.BoardPresence < mid.BoardPresence-0.05 {
		t.Errorf("Control BoardPresence (%.2f) should be neutral (≈ midrange %.2f), not deprioritized",
			w.BoardPresence, mid.BoardPresence)
	}
	if w.BoardPresence > mid.BoardPresence+0.05 {
		t.Errorf("Control BoardPresence (%.2f) should be neutral, not boosted above midrange (%.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
	// Sanity: aggro should still out-prioritize board over control.
	if w.BoardPresence >= aggro.BoardPresence {
		t.Errorf("Control BoardPresence (%.2f) should remain below aggro (%.2f) — neutral, not pressuring",
			w.BoardPresence, aggro.BoardPresence)
	}
}

// TestControlWeights_StackInteractionStillHigh pins that the retune
// didn't accidentally erode the existing Control identity dial. StackInteraction
// is the defining Control axis and must remain the second-highest weight
// (tied with ThreatExposure).
func TestControlWeights_StackInteractionStillHigh(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeControl)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.StackInteraction < 1.5 {
		t.Errorf("Control StackInteraction should remain ≥ 1.5; got %.2f", w.StackInteraction)
	}
	if w.StackInteraction <= mid.StackInteraction {
		t.Errorf("Control StackInteraction (%.2f) should exceed midrange (%.2f)",
			w.StackInteraction, mid.StackInteraction)
	}
}
