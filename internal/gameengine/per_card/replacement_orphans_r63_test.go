package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// replacement_orphans_r63_test.go — regression pins for the r63
// replacement-orphan tail per_card handlers. Each card's conditional
// "instead" clause parsed to an inert parsed_tail; these tests assert the
// conditional now fires (and the base still applies when it doesn't).

func bigCreature(gs *gameengine.GameState, seat int, name string, pow, tough int) *gameengine.Permanent {
	p := addPerm(gs, seat, name, "creature")
	p.Card.BasePower, p.Card.BaseToughness = pow, tough
	return p
}

func onBattlefield(gs *gameengine.GameState, perm *gameengine.Permanent) bool {
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == perm {
				return true
			}
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Join the Dead — base -5/-5; Descend 4 → -10/-10.
// -----------------------------------------------------------------------------

func TestJoinTheDead_BaseMinusFive(t *testing.T) {
	gs := newGame(t, 2)
	c := bigCreature(gs, 1, "Big Beast", 6, 6) // graveyard empty → -5/-5
	card := addCard(gs, 0, "Join the Dead", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if c.Toughness() != 1 {
		t.Fatalf("base -5/-5 on a 6/6 should leave toughness 1, got %d", c.Toughness())
	}
}

func TestJoinTheDead_Descend4_MinusTen(t *testing.T) {
	gs := newGame(t, 2)
	// 4 permanent cards in the caster's graveyard → Descend 4 → -10/-10.
	for _, n := range []string{"gy1", "gy2", "gy3", "gy4"} {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
			&gameengine.Card{Name: n, Owner: 0, Types: []string{"creature"}})
	}
	c := bigCreature(gs, 1, "Huge Beast", 8, 8)

	card := addCard(gs, 0, "Join the Dead", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	// 8/8 - 10/-10 → -2/-2, dies to SBA.
	if c.Toughness() != -2 {
		t.Fatalf("Descend 4 should apply -10/-10 (toughness -2 on an 8/8), got %d", c.Toughness())
	}
	gameengine.StateBasedActions(gs)
	for _, p := range gs.Seats[1].Battlefield {
		if p == c {
			t.Fatal("a creature reduced below 0 toughness must die to SBA")
		}
	}
}

// -----------------------------------------------------------------------------
// Anoint with Affliction — base exiles MV≤3; Corrupted exiles any MV.
// -----------------------------------------------------------------------------

func TestAnoint_BaseOnlyHitsSmallMV(t *testing.T) {
	gs := newGame(t, 2)
	big := bigCreature(gs, 1, "Colossus", 8, 8)
	big.Card.CMC = 8 // MV 8, opponent NOT corrupted → ineligible

	card := addCard(gs, 0, "Anoint with Affliction", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	// Big MV-8 creature must survive (no eligible target → fizzle).
	if !onBattlefield(gs, big) {
		t.Fatal("Anoint base must NOT exile an MV-8 creature without Corrupted")
	}
}

func TestAnoint_Corrupted_ExilesAnyMV(t *testing.T) {
	gs := newGame(t, 2)
	big := bigCreature(gs, 1, "Colossus", 8, 8)
	big.Card.CMC = 8
	gs.Seats[1].PoisonCounters = 3 // Corrupted

	card := addCard(gs, 0, "Anoint with Affliction", "instant")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if onBattlefield(gs, big) {
		t.Fatal("Corrupted Anoint must exile a high-MV creature against a poisoned controller")
	}
	inExile := false
	for _, c := range gs.Seats[1].Exile {
		if c == big.Card {
			inExile = true
		}
	}
	if !inExile {
		t.Fatal("the exiled creature must be in its owner's exile")
	}
}

// -----------------------------------------------------------------------------
// Withering Curse — base -2/-2 sweep; Infusion → destroy all.
// -----------------------------------------------------------------------------

func TestWitheringCurse_BaseMinusTwoSweep(t *testing.T) {
	gs := newGame(t, 2)
	a := bigCreature(gs, 0, "Mine", 3, 3)
	b := bigCreature(gs, 1, "Theirs", 1, 1)

	card := addCard(gs, 0, "Withering Curse", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if a.Toughness() != 1 {
		t.Errorf("symmetric -2/-2 should leave own 3/3 at toughness 1, got %d", a.Toughness())
	}
	gameengine.StateBasedActions(gs)
	if onBattlefield(gs, b) {
		t.Error("a 1/1 must die to the -2/-2 sweep")
	}
}

func TestWitheringCurse_Infusion_DestroysAll(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Turn.LifeGained = 3 // gained life this turn → Infusion
	a := bigCreature(gs, 0, "Mine", 5, 5)
	b := bigCreature(gs, 1, "Theirs", 5, 5)

	card := addCard(gs, 0, "Withering Curse", "sorcery")
	gameengine.InvokeResolveHook(gs, &gameengine.StackItem{Controller: 0, Card: card})

	if onBattlefield(gs, a) || onBattlefield(gs, b) {
		t.Fatal("Infusion Withering Curse must destroy ALL creatures (even big ones)")
	}
}
