package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Boros Charm: modal spell, inert (parsed_tail). The per_card handler picks
// the burn mode — 4 damage to the lowest-life opponent's face.

func TestBorosCharm_DealsFourToLowestLifeOpponent(t *testing.T) {
	gs := newGame(t, 3)
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 12 // lowest

	card := addCard(gs, 0, "Boros Charm", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	gameengine.InvokeResolveHook(gs, item)

	if gs.Seats[2].Life != 8 {
		t.Fatalf("expected lowest-life opponent (seat 2) to take 4 → 8, got %d", gs.Seats[2].Life)
	}
	if gs.Seats[1].Life != 30 {
		t.Fatalf("higher-life opponent should be untouched, got %d", gs.Seats[1].Life)
	}
}

func TestBorosCharm_NoLivingOpponentNoop(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Lost = true
	card := addCard(gs, 0, "Boros Charm", "instant")
	item := &gameengine.StackItem{Controller: 0, Card: card}
	// Should not panic; no living opponent to hit.
	gameengine.InvokeResolveHook(gs, item)
}
