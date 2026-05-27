package gameengine

import (
	"testing"
)

// cumulative_upkeep_r60_test.go — CR §702.24a regression.
//
// CR §702.24a: "Cumulative upkeep [cost] means 'At the beginning of
// your upkeep, if this permanent is on the battlefield, put an age
// counter on this permanent. Then you may pay [cost] for each age
// counter on it. If you don't, sacrifice it.'"
//
// `ApplyCumulativeUpkeep` (keywords_batch.go) implemented the per-perm
// effect (add counter, then pay-or-sacrifice) and the Tombstone
// Stairwell per_card handler stamped `Flags["cumulative_upkeep_cost"]
// = 2` at ETB to opt in. But nothing in the upkeep step actually
// CALLED ApplyCumulativeUpkeep — the helper was dead code, so every
// cumulative-upkeep permanent (Tombstone Stairwell, Glacial Chasm,
// Drought, Phyrexian Marauder, every other CU card) accumulated zero
// age counters and never paid the escalating cost. The keyword was
// inert.
//
// The fix wires ApplyCumulativeUpkeep into FirePhaseTriggers when
// step=="upkeep", scoped to the ACTIVE seat's battlefield to match
// "at the beginning of your upkeep" controller-scoping per §702.24a.
//
// Three regressions here:
//
//  1. TestCumulativeUpkeep_AgeCounterAddedEachUpkeep — calling the
//     upkeep phase trigger fan-out adds an age counter per call and
//     debits the controller's mana pool accordingly. Two upkeep cycles
//     accumulate to 2 age counters and cost 1×2 + 2×2 = 6 mana on a
//     Tombstone-Stairwell-style perm (cost=2 per age counter).
//
//  2. TestCumulativeUpkeep_UnpaidUpkeepSacrifices — when the active
//     seat lacks mana to pay, the permanent is sacrificed (§702.24a's
//     "if you don't, sacrifice it" clause). The permanent must leave
//     the battlefield and land in its owner's graveyard.
//
//  3. TestCumulativeUpkeep_NonActiveSeatDoesNotFire — cumulative
//     upkeep is "at the beginning of YOUR upkeep" per §702.24a; a
//     non-active seat's cumulative-upkeep permanent must NOT
//     accumulate age counters when the active seat's upkeep fires.
//     Defends against the wiring scanning all seats' battlefields.

func TestCumulativeUpkeep_AgeCounterAddedEachUpkeep(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase, gs.Step = "beginning", "upkeep"
	gs.Seats[0].ManaPool = 10

	perm := addBattlefield(gs, 0, "Tombstone Stairwell", 0, 0, "enchantment", "world")
	perm.Flags["cumulative_upkeep_cost"] = 2 // {1}{B} CMC

	// First upkeep — should add 1 age counter, pay 1*2 = 2 mana.
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if perm.Counters["age"] != 1 {
		t.Fatalf("after first upkeep: age=%d, want 1", perm.Counters["age"])
	}
	if gs.Seats[0].ManaPool != 8 {
		t.Fatalf("after first upkeep: mana=%d, want 8 (10 - 1*2)", gs.Seats[0].ManaPool)
	}

	// Second upkeep — should add another age counter (total 2), pay 2*2 = 4 more mana.
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if perm.Counters["age"] != 2 {
		t.Fatalf("after second upkeep: age=%d, want 2", perm.Counters["age"])
	}
	if gs.Seats[0].ManaPool != 4 {
		t.Fatalf("after second upkeep: mana=%d, want 4 (8 - 2*2)", gs.Seats[0].ManaPool)
	}

	// Permanent still alive (cost was paid).
	if !permanentOnBattlefield(gs, perm) {
		t.Fatal("permanent should still be on battlefield after paid cumulative upkeep")
	}
}

func TestCumulativeUpkeep_UnpaidUpkeepSacrifices(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase, gs.Step = "beginning", "upkeep"
	gs.Seats[0].ManaPool = 1 // insufficient — needs 2 (cost) * 1 (counter) = 2

	perm := addBattlefield(gs, 0, "Tombstone Stairwell", 0, 0, "enchantment", "world")
	perm.Flags["cumulative_upkeep_cost"] = 2

	FirePhaseTriggers(gs, "beginning", "upkeep")

	if permanentOnBattlefield(gs, perm) {
		t.Fatal("permanent should have been sacrificed when cumulative upkeep cost couldn't be paid")
	}
	// Sacrificed card must be in owner's graveyard.
	foundInGraveyard := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == perm.Card {
			foundInGraveyard = true
			break
		}
	}
	if !foundInGraveyard {
		t.Errorf("sacrificed cumulative-upkeep permanent should be in owner's graveyard")
	}
	// Mana should be untouched on the sacrifice path (§702.24a: "if you
	// don't, sacrifice it" — the cost wasn't paid).
	if gs.Seats[0].ManaPool != 1 {
		t.Errorf("mana should be untouched on sacrifice path; got %d", gs.Seats[0].ManaPool)
	}
}

func TestCumulativeUpkeep_NonActiveSeatDoesNotFire(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Active = 0
	gs.Phase, gs.Step = "beginning", "upkeep"
	gs.Seats[1].ManaPool = 10

	// Cumulative-upkeep permanent on seat 1 (NON-active).
	perm := addBattlefield(gs, 1, "Tombstone Stairwell", 0, 0, "enchantment", "world")
	perm.Flags["cumulative_upkeep_cost"] = 2

	FirePhaseTriggers(gs, "beginning", "upkeep")

	// Seat 1's permanent must NOT have an age counter — it's seat 0's
	// upkeep, not seat 1's. Per CR §702.24a "at the beginning of YOUR
	// upkeep" scopes the trigger to the controller of the permanent
	// only when it's that player's turn.
	if perm.Counters["age"] != 0 {
		t.Errorf("seat 1's cumulative-upkeep perm fired on seat 0's upkeep; age=%d, want 0",
			perm.Counters["age"])
	}
	if gs.Seats[1].ManaPool != 10 {
		t.Errorf("seat 1's mana should be untouched on seat 0's upkeep; got %d",
			gs.Seats[1].ManaPool)
	}
}
