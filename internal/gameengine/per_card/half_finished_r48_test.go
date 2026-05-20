package per_card

import (
	"testing"
)

// Regression for fix #2 in dev/half-finished-r48:
// Captain America's Throw activation used to pay {3} via a "defensive
// top-up" — `if seat.ManaPool >= 3 { seat.ManaPool -= 3 }` — which let
// the activation succeed for free when the caller had <3 mana. Match
// the Erebos / Tasigur cost-gate pattern: insufficient mana must
// emitFail and leave state untouched.
func TestHalfFinishedR48_CaptainAmerica_ThrowFailsWithoutMana(t *testing.T) {
	gs := newGame(t, 2)
	cap := addPerm(gs, 0, "Captain America, First Avenger", "creature")
	eq := addPerm(gs, 0, "Shield", "artifact", "equipment")
	eq.Card.CMC = 2
	eq.AttachedTo = cap

	gs.Seats[0].ManaPool = 2 // not enough for {3}
	opp := gs.Seats[1]
	startingOppLife := opp.Life

	captainAmericaFirstAvengerActivate(gs, cap, 0, nil)

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("mana pool should be untouched when cost fails; got %d", gs.Seats[0].ManaPool)
	}
	if eq.AttachedTo != cap {
		t.Errorf("equipment should remain attached when activation fails")
	}
	if opp.Life != startingOppLife {
		t.Errorf("opponent life should be untouched when activation fails; got %d", opp.Life)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

// Happy-path regression: with enough mana, the activation still pays
// the cost, unattaches the equipment, and deals damage. Guards against
// the cost-gate rewrite over-restricting valid activations.
func TestHalfFinishedR48_CaptainAmerica_ThrowSucceedsWithMana(t *testing.T) {
	gs := newGame(t, 2)
	cap := addPerm(gs, 0, "Captain America, First Avenger", "creature")
	eq := addPerm(gs, 0, "Shield", "artifact", "equipment")
	eq.Card.CMC = 2
	eq.AttachedTo = cap

	gs.Seats[0].ManaPool = 5
	opp := gs.Seats[1]
	startingOppLife := opp.Life

	captainAmericaFirstAvengerActivate(gs, cap, 0, nil)

	if gs.Seats[0].ManaPool != 2 {
		t.Errorf("expected mana 5-3=2 after Throw; got %d", gs.Seats[0].ManaPool)
	}
	if eq.AttachedTo == cap {
		t.Errorf("equipment should be unattached after Throw")
	}
	// Equipment MV 2 → 2 damage on opponent. Captain America targets the
	// lowest-life opponent; with only one opponent (seat 1) the full 2 lands.
	// (Lethal-vs-spread branch is irrelevant when opp.Life > MV.)
	dmgDealt := startingOppLife - opp.Life
	if dmgDealt != 2 {
		t.Errorf("expected 2 damage to opponent (eq MV=2); got %d", dmgDealt)
	}
}

// Ensure the cost gate enforces "no equipment" before "no mana" — if
// neither is satisfied we should fail on the equipment shape (matches
// the prior emit_fail reason).
func TestHalfFinishedR48_CaptainAmerica_NoEquipmentFailsFirst(t *testing.T) {
	gs := newGame(t, 2)
	cap := addPerm(gs, 0, "Captain America, First Avenger", "creature")
	gs.Seats[0].ManaPool = 5 // plenty of mana, just no equipment

	captainAmericaFirstAvengerActivate(gs, cap, 0, nil)

	if gs.Seats[0].ManaPool != 5 {
		t.Errorf("mana should be untouched when no equipment; got %d", gs.Seats[0].ManaPool)
	}
	if hasEvent(gs, "per_card_failed") < 1 {
		t.Errorf("expected per_card_failed event")
	}
}

