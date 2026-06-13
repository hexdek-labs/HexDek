package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// glcover_etbdraw_r60_test.go — regression pins for shard G-L ETB/cast
// draw triggers wired through gameengine.DrawN (Lord of Change,
// Liliana's Standard Bearer, Kozilek). Each previously parsed to an
// inert raw-text node and drew ZERO cards.

func TestLordOfChange_ETBDrawsThree(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	preHand := len(gs.Seats[0].Hand)
	perm := addPerm(gs, 0, "Lord of Change", "creature")
	gameengine.InvokeETBHook(gs, perm)
	if got := len(gs.Seats[0].Hand) - preHand; got != 3 {
		t.Errorf("drew %d, want 3", got)
	}
}

func TestLilianasStandardBearer_DrawsPerCreaturesDied(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	gs.Seats[0].Turn.CreaturesDied = 4
	preHand := len(gs.Seats[0].Hand)
	perm := addPerm(gs, 0, "Liliana's Standard Bearer", "creature")
	gameengine.InvokeETBHook(gs, perm)
	if got := len(gs.Seats[0].Hand) - preHand; got != 4 {
		t.Errorf("drew %d, want 4 (creatures died this turn)", got)
	}
}

func TestLilianasStandardBearer_ZeroDeathsNoDraw(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	preHand := len(gs.Seats[0].Hand)
	perm := addPerm(gs, 0, "Liliana's Standard Bearer", "creature")
	gameengine.InvokeETBHook(gs, perm)
	if got := len(gs.Seats[0].Hand) - preHand; got != 0 {
		t.Errorf("drew %d, want 0", got)
	}
}

func TestKozilek_CastDrawsUpToSeven(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	// Hand has 2 cards -> draw 5 to reach 7.
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "A", Owner: 0}, {Name: "B", Owner: 0},
	}
	card := addCard(gs, 0, "Kozilek, the Great Distortion", "creature")
	gameengine.InvokeCastHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if got := len(gs.Seats[0].Hand); got != 7 {
		t.Errorf("hand = %d, want 7 (drew 5)", got)
	}
}

func TestKozilek_FullHandNoDraw(t *testing.T) {
	gs := newGame(t, 2)
	fillLibrary(gs, 0, 10)
	gs.Seats[0].Hand = make([]*gameengine.Card, 8) // 8 >= 7
	for i := range gs.Seats[0].Hand {
		gs.Seats[0].Hand[i] = &gameengine.Card{Name: "X", Owner: 0}
	}
	card := addCard(gs, 0, "Kozilek, the Great Distortion", "creature")
	gameengine.InvokeCastHook(gs, &gameengine.StackItem{Controller: 0, Card: card})
	if got := len(gs.Seats[0].Hand); got != 8 {
		t.Errorf("hand = %d, want 8 (no draw at 7+)", got)
	}
}
