package gameengine

// r63 — loot / rummage / connive ordering + cardinality audit.
//
// These mechanics are "legal-state" rules correctness — fuzz invariants can't
// see them (looting twice or milling instead of discarding violates no
// conservation/legality invariant; it's just the wrong card economy).
//
// Bugs fixed (regressions below):
//   1. draw_discard_effect (loot, 25 corpus cards) milled the LIBRARY instead
//      of discarding from HAND.
//   2. the "connive" keyword_action arm milled + added +1/+1 UNCONDITIONALLY +
//      ignored N; now delegates to the canonical Connive (draw N, discard N
//      from hand, +1/+1 per NONLAND discarded).
//   3. the reResDiscardDraw (rummage) text path milled the library instead of
//      discarding from hand.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func lootTestGame(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(1)), nil)
}

func creatureSrc(seat int, name string) *Permanent {
	return &Permanent{
		Card:       &Card{Name: name, Owner: seat, Types: []string{"creature"}},
		Controller: seat, Owner: seat,
		Counters: map[string]int{}, Flags: map[string]int{},
	}
}

func libHas(lib []*Card, name string) bool {
	for _, c := range lib {
		if c != nil && c.Name == name {
			return true
		}
	}
	return false
}

// Bug 1: loot draws from library to hand, then DISCARDS FROM HAND — it must NOT
// mill the library's second card.
func TestLoot_DrawDiscardEffect_DiscardsFromHandNotMill(t *testing.T) {
	gs := lootTestGame(t)
	s := gs.Seats[0]
	s.Library = []*Card{{Name: "L1", Owner: 0}, {Name: "L2_wouldbe_milled", Owner: 0}, {Name: "L3", Owner: 0}}
	s.Hand = []*Card{{Name: "H1", Owner: 0, Types: []string{"land"}}, {Name: "H2", Owner: 0}}

	ResolveEffect(gs, creatureSrc(0, "Steamcore Scholar"), &gameast.ModificationEffect{ModKind: "draw_discard_effect"})

	// Only the drawn card (L1) left the library; L2 must NOT have been milled.
	if len(s.Library) != 2 {
		t.Fatalf("loot milled the library: want 2 cards left (L1 drawn), got %d", len(s.Library))
	}
	if !libHas(s.Library, "L2_wouldbe_milled") {
		t.Fatalf("loot milled L2 from the library instead of discarding from hand")
	}
	// Exactly one discard, and it came from HAND (not a library card).
	if len(s.Graveyard) != 1 {
		t.Fatalf("loot: want exactly 1 discard in graveyard, got %d", len(s.Graveyard))
	}
	if libHas(s.Graveyard, "L2_wouldbe_milled") || libHas(s.Graveyard, "L3") {
		t.Fatalf("loot discarded a LIBRARY card; the discard must come from hand")
	}
}

// Bug 2: connive keyword_action — draw N, discard N from hand, +1/+1 per NONLAND
// discarded (not unconditional, not N-ignoring, not a mill).
func TestConnive_KeywordAction_DrawNDiscardNNonlandCounters(t *testing.T) {
	gs := lootTestGame(t)
	s := gs.Seats[0]
	// Top two library cards are nonland creatures — they get drawn then
	// discarded (Connive discards from the hand tail = the just-drawn cards),
	// so two nonlands are discarded → exactly two +1/+1 counters.
	s.Library = []*Card{
		{Name: "C1", Owner: 0, Types: []string{"creature"}},
		{Name: "C2", Owner: 0, Types: []string{"creature"}},
		{Name: "C3", Owner: 0},
	}
	s.Hand = []*Card{{Name: "HandLand", Owner: 0, Types: []string{"land"}}}
	src := creatureSrc(0, "Conniver")

	ResolveEffect(gs, src, &gameast.ModificationEffect{ModKind: "keyword_action", Args: []interface{}{"connive 2"}})

	if got := src.Counters["+1/+1"]; got != 2 {
		t.Fatalf("connive 2: want 2 +1/+1 counters (2 nonlands discarded), got %d", got)
	}
	// Two discards landed in the graveyard (from hand), nothing milled: C3 (the
	// 3rd library card) was never drawn and must remain in the library.
	if len(s.Graveyard) != 2 {
		t.Fatalf("connive 2: want 2 cards discarded to graveyard, got %d", len(s.Graveyard))
	}
	if !libHas(s.Library, "C3") {
		t.Fatalf("connive milled/over-drew the library: C3 should still be there")
	}
}

// Bug 3: rummage text path discards from HAND, then draws — must not mill.
func TestRummage_DiscardDrawText_DiscardsFromHand(t *testing.T) {
	gs := lootTestGame(t)
	s := gs.Seats[0]
	s.Library = []*Card{{Name: "D1", Owner: 0}, {Name: "D2", Owner: 0}, {Name: "D3", Owner: 0}, {Name: "D4", Owner: 0}}
	s.Hand = []*Card{{Name: "H1", Owner: 0}, {Name: "H2", Owner: 0}, {Name: "H3", Owner: 0}}
	src := creatureSrc(0, "Rummager")

	if !resolveResidualByText(gs, src, "discard two cards, then draw two cards") {
		t.Fatalf("resolveResidualByText did not handle the discard-then-draw text")
	}

	// Two cards discarded FROM HAND (Turn.Discarded only bumps via DiscardCard).
	if s.Turn.Discarded != 2 {
		t.Fatalf("rummage: want Turn.Discarded==2 (real hand discard), got %d (mill bypasses it)", s.Turn.Discarded)
	}
	// Graveyard holds the two HAND cards, not milled library cards.
	if len(s.Graveyard) != 2 {
		t.Fatalf("rummage: want 2 cards in graveyard, got %d", len(s.Graveyard))
	}
	if libHas(s.Graveyard, "D1") || libHas(s.Graveyard, "D2") || libHas(s.Graveyard, "D3") || libHas(s.Graveyard, "D4") {
		t.Fatalf("rummage discarded a LIBRARY card (milled) instead of from hand")
	}
}
