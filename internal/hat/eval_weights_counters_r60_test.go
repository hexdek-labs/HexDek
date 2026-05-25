package hat

import (
	"testing"
)

// eval_weights_counters_r60_test.go — Counters Matter archetype retune.
// Pre-fix the profile looked like "midrange + slight BoardPresence
// nudge." This test pins the three signature dials off generic:
// BoardPresence (engine + wincon, compounds non-linearly via doublers
// and proliferate), CommanderProgress (Atraxa/Ezuri/Pir+Toothy/Hapatra
// ARE the counter payoff), and ComboProximity (infect 10-poison +
// Walking Ballista / Hangarback / Kalonian Hydra + Doubling Season).

// -----------------------------------------------------------------------------
// BoardPresence is the engine AND the wincon — must be the dominant dial.
// -----------------------------------------------------------------------------

func TestCountersMatterWeights_BoardPresenceRemainsHighest(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeCountersMatter)
	a := w.AsArray()
	idxBoardPresence := 0 // BoardPresence slot
	best := -1.0
	bestIdx := -1
	for i, v := range a {
		if v > best {
			best = v
			bestIdx = i
		}
	}
	if bestIdx != idxBoardPresence {
		t.Fatalf("CountersMatter's highest weight should be BoardPresence (idx %d), got idx %d val %.2f",
			idxBoardPresence, bestIdx, best)
	}
}

func TestCountersMatterWeights_BoardPresenceBoosted(t *testing.T) {
	// Each creature with +1/+1 counters compounds non-linearly via
	// doublers (Hardened Scales, Branching Evolution, Doubling Season)
	// and proliferate. The board is both the engine and the wincon —
	// 1.2 was a midrange-tier nudge, not an archetype signature.
	w := DefaultWeightsForArchetype(ArchetypeCountersMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.BoardPresence < 1.5 {
		t.Errorf("CountersMatter BoardPresence should be ≥ 1.5 (engine + wincon, non-linear compound); got %.2f",
			w.BoardPresence)
	}
	if w.BoardPresence <= mid.BoardPresence {
		t.Errorf("CountersMatter BoardPresence (%.2f) should exceed midrange (%.2f)",
			w.BoardPresence, mid.BoardPresence)
	}
}

// -----------------------------------------------------------------------------
// CommanderProgress: Atraxa, Ezuri, Pir+Toothy, Hapatra, Skullbriar,
// Marchesa, Vorel, Animar, Ghave — the anchor commanders ARE the
// counter payoff, not "play when convenient" curve plays.
// -----------------------------------------------------------------------------

func TestCountersMatterWeights_CommanderProgressBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeCountersMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.CommanderProgress < 1.3 {
		t.Errorf("CountersMatter CommanderProgress should be ≥ 1.3 (Atraxa/Ezuri/Pir+Toothy ARE the payoff); got %.2f",
			w.CommanderProgress)
	}
	if w.CommanderProgress <= mid.CommanderProgress {
		t.Errorf("CountersMatter CommanderProgress (%.2f) should exceed midrange (%.2f)",
			w.CommanderProgress, mid.CommanderProgress)
	}
}

// -----------------------------------------------------------------------------
// ComboProximity: infect (Skithiryx / Phyrexian Crusader / Triumph of
// the Hordes 10-poison) and the +1/+1 finishers (Walking Ballista,
// Hangarback Walker, Kalonian Hydra + Doubling Season one-shot,
// Animar storm) are real win lines, not value-engine noise.
// -----------------------------------------------------------------------------

func TestCountersMatterWeights_ComboProximityBoosted(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeCountersMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w.ComboProximity < 1.2 {
		t.Errorf("CountersMatter ComboProximity should be ≥ 1.2 (infect + Walking Ballista + Hydra+DS); got %.2f",
			w.ComboProximity)
	}
	if w.ComboProximity <= mid.ComboProximity {
		t.Errorf("CountersMatter ComboProximity (%.2f) should exceed midrange (%.2f)",
			w.ComboProximity, mid.ComboProximity)
	}
}

// -----------------------------------------------------------------------------
// Profile must be off generic — sanity check that the retune actually
// differentiates from midrange across the three signature dials.
// -----------------------------------------------------------------------------

func TestCountersMatterWeights_OffMidrange(t *testing.T) {
	w := DefaultWeightsForArchetype(ArchetypeCountersMatter)
	mid := DefaultWeightsForArchetype(ArchetypeMidrange)
	if w == mid {
		t.Fatalf("CountersMatter weights should differ from midrange")
	}
	// All three signature dials must clear midrange — no profile drift
	// back into "midrange + BoardPresence nudge" shape.
	signatureLifts := 0
	if w.BoardPresence > mid.BoardPresence {
		signatureLifts++
	}
	if w.CommanderProgress > mid.CommanderProgress {
		signatureLifts++
	}
	if w.ComboProximity > mid.ComboProximity {
		signatureLifts++
	}
	if signatureLifts < 3 {
		t.Errorf("expected all 3 signature dials (BoardPresence / CommanderProgress / ComboProximity) above midrange, got %d", signatureLifts)
	}
}
