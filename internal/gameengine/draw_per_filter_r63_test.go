package gameengine

// r63 OUTCOME phase-3 engine regression: the draw_per ModificationEffect
// arm ignored its filter arg — "draw a card for each TAPPED CREATURE
// TARGET OPPONENT CONTROLS" (Theft of Dreams, Borrowing 100,000 Arrows)
// counted the caster's OWN creatures, and a zero count still floored to
// 1. countPerFilter now drives recognized filters exactly.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestDrawPer_FilterDrivesCount(t *testing.T) {
	gs := newFixtureGame(t)
	addBattlefield(gs, 0, "Own Bear A", 2, 2, "creature")
	addBattlefield(gs, 0, "Own Bear B", 2, 2, "creature")
	tappedOpp := addBattlefield(gs, 1, "Opp Bear", 2, 2, "creature")
	tappedOpp.Tapped = true
	addLibrary(gs, 0, "L1", "L2", "L3", "L4", "L5")
	src := addBattlefield(gs, 0, "Theft Source", 1, 1, "creature")

	// "tapped creature target opponent controls" — exactly 1 match.
	hand := len(gs.Seats[0].Hand)
	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "draw_per",
		Args:    []interface{}{"tapped creature target opponent controls"},
	})
	if got := len(gs.Seats[0].Hand) - hand; got != 1 {
		t.Fatalf("filtered draw_per must draw per MATCHING object: want 1, got %d", got)
	}

	// Untap the opponent's creature: zero matches must draw ZERO (the
	// old arm floored to 1).
	tappedOpp.Tapped = false
	hand = len(gs.Seats[0].Hand)
	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "draw_per",
		Args:    []interface{}{"tapped creature target opponent controls"},
	})
	if got := len(gs.Seats[0].Hand) - hand; got != 0 {
		t.Fatalf("zero matches must draw zero (no floor): got %d", got)
	}

	// The dominant default still works: own creatures (2 bears + 1 src).
	hand = len(gs.Seats[0].Hand)
	ResolveEffect(gs, src, &gameast.ModificationEffect{
		ModKind: "draw_per",
		Args:    []interface{}{"creature you control"},
	})
	if got := len(gs.Seats[0].Hand) - hand; got != 3 {
		t.Fatalf("creature-you-control draw_per: want 3, got %d", got)
	}
}
