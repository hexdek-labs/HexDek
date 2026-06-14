package per_card

// r63 — Fearless Swashbuckler's "draw three cards, then discard two cards" loot
// routed its discard through a raw hand→graveyard MoveCard instead of the
// DiscardCard chokepoint, silently bypassing §702.34 Madness, card_discarded
// observer triggers, and the per-turn discard-count stat. A loot's "discard" is
// a real discard, not a silent zone move.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestFearlessSwashbuckler_DiscardGoesThroughChokepoint(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Hand = []*gameengine.Card{
		{Name: "Card A", Owner: 0, Types: []string{"creature", "cost:5"}},
		{Name: "Card B", Owner: 0, Types: []string{"instant", "cost:3"}},
		{Name: "Card C", Owner: 0, Types: []string{"sorcery", "cost:2"}},
		{Name: "Card D", Owner: 0, Types: []string{"creature", "cost:1"}},
	}

	names := fearlessSwashbucklerDiscardTwo(gs, 0, "test")

	if len(names) != 2 {
		t.Fatalf("expected 2 cards discarded, got %d", len(names))
	}
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Fatalf("expected 2 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	// Turn.Discarded is incremented ONLY by the DiscardCard chokepoint — a raw
	// MoveCard("hand","graveyard") leaves it at 0. This is the load-bearing
	// assertion that the discard is a real discard.
	if got := gs.Seats[0].Turn.Discarded; got != 2 {
		t.Fatalf("discard bypassed the DiscardCard chokepoint: Turn.Discarded=%d, want 2", got)
	}
}
