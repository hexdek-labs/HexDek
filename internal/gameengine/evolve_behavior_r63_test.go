package gameengine

import "testing"

// evolve_behavior_r63_test.go — behavioral probe for Evolve (CR §702.100).
// Before r63 the dedicated FireEvolveTriggers was UNWIRED (no caller) and used
// raw AddCounter (bypassing doublers + counter_placed). These pin the wiring
// and the properties the existing direct-call tests left uncovered.

// (wiring) Evolve fires through the live observer-ETB chokepoint, not only via a
// direct FireEvolveTriggers call. fireObserverETBTriggers is the single path
// both the cast and non-cast ETB routes funnel through.
func TestEvolve_FiresThroughObserverETBPath(t *testing.T) {
	gs := newKWCombatGame(t)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 1, 1, "evolve", "creature")
	entering := addKWCombatBattlefield(gs, 0, "Big Beast", 3, 3, "creature")

	FireObserverETBTriggers(gs, entering)

	if evolver.Counters["+1/+1"] != 1 {
		t.Errorf("evolve must fire through the live ETB path; got %d counters", evolver.Counters["+1/+1"])
	}
}

// (b) Exactly ONE +1/+1 counter per qualifying ETB, even when BOTH power and
// toughness are greater.
func TestEvolve_OneCounterPerETBNotPerStat(t *testing.T) {
	gs := newKWCombatGame(t)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 1, 1, "evolve", "creature")
	entering := addKWCombatBattlefield(gs, 0, "Bigger", 5, 5, "creature")

	FireEvolveTriggers(gs, 0, entering)

	if evolver.Counters["+1/+1"] != 1 {
		t.Errorf("(b) one ETB with both stats greater should add exactly 1 counter, got %d", evolver.Counters["+1/+1"])
	}
}

// (c) The comparison uses the evolve creature's CURRENT (counter-modified)
// stats — it gets harder to trigger as it grows.
func TestEvolve_UsesCurrentStats(t *testing.T) {
	gs := newKWCombatGame(t)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 2, 2, "evolve", "creature")
	evolver.Counters["+1/+1"] = 1 // now effectively 3/3

	// A 3/3 enters — NOT strictly greater than the current 3/3 → no trigger.
	e1 := addKWCombatBattlefield(gs, 0, "ThreeThree", 3, 3, "creature")
	FireEvolveTriggers(gs, 0, e1)
	if evolver.Counters["+1/+1"] != 1 {
		t.Errorf("(c) a 3/3 must not trigger an evolver already at 3/3; counters=%d", evolver.Counters["+1/+1"])
	}

	// A 4/4 enters — greater than the current 3/3 → triggers.
	e2 := addKWCombatBattlefield(gs, 0, "FourFour", 4, 4, "creature")
	FireEvolveTriggers(gs, 0, e2)
	if evolver.Counters["+1/+1"] != 2 {
		t.Errorf("(c) a 4/4 should trigger the 3/3 evolver; counters=%d (want 2)", evolver.Counters["+1/+1"])
	}
}

// (d) The +1/+1 counter respects counter doublers (Doubling Season).
func TestEvolve_CounterRespectsDoublers(t *testing.T) {
	gs := newKWCombatGame(t)
	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 1, 1, "evolve", "creature")
	entering := addKWCombatBattlefield(gs, 0, "Big Beast", 3, 3, "creature")

	FireEvolveTriggers(gs, 0, entering)

	if evolver.Counters["+1/+1"] != 2 {
		t.Errorf("(d) evolve counter must double under Doubling Season → 2, got %d", evolver.Counters["+1/+1"])
	}
}

// (f) A creature entering with EQUAL stats does NOT trigger (strictly greater).
func TestEvolve_EqualStatsNoTrigger(t *testing.T) {
	gs := newKWCombatGame(t)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 2, 2, "evolve", "creature")
	entering := addKWCombatBattlefield(gs, 0, "Equal", 2, 2, "creature")

	FireEvolveTriggers(gs, 0, entering)

	if evolver.Counters["+1/+1"] != 0 {
		t.Errorf("(f) equal 2/2 must NOT trigger evolve (strictly greater required); counters=%d", evolver.Counters["+1/+1"])
	}
}

// (e) An opponent's creature entering does not trigger your evolve creature.
func TestEvolve_OpponentETBDoesNotTrigger(t *testing.T) {
	gs := newKWCombatGame(t)
	evolver := addKWCombatBattlefieldWithKeyword(gs, 0, "Experiment One", 1, 1, "evolve", "creature")
	oppCreature := addKWCombatBattlefield(gs, 1, "Enemy Beast", 5, 5, "creature")

	FireObserverETBTriggers(gs, oppCreature)

	if evolver.Counters["+1/+1"] != 0 {
		t.Errorf("(e) an opponent's creature must not trigger your evolve; counters=%d", evolver.Counters["+1/+1"])
	}
}
