package gameengine

// r63 — Madness (CR §702.34) audit. Ties to the discard-was-mill find: a
// madness discard must STILL count as a discard (replace the destination with
// exile, not skip the discard), and the window must resolve.

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func madGame() *GameState {
	gs := NewGameState(2, rand.New(rand.NewSource(2)), nil)
	for _, s := range gs.Seats {
		s.Life = 40
	}
	return gs
}

func madCard(seat, cost int, name string) *Card {
	return &Card{
		Name: name, Owner: seat, Types: []string{"instant"},
		AST: &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "madness", Args: []interface{}{cost}},
		}},
	}
}

func cardsContain(z []*Card, c *Card) bool {
	for _, x := range z {
		if x == c {
			return true
		}
	}
	return false
}

// (a) discard replaces graveyard with exile + opens the cast window;
// (d) it STILL counts as a discard (Turn.Discarded++, card_discarded fired).
func TestMadness_DiscardReplacedWithExileStillCounts(t *testing.T) {
	gs := madGame()
	m := madCard(0, 2, "Fiery Temper")
	gs.Seats[0].Hand = []*Card{m}

	DiscardCard(gs, m, 0)

	if gs.Seats[0].Turn.Discarded != 1 {
		t.Fatalf("madness discard must still COUNT as a discard; Turn.Discarded=%d", gs.Seats[0].Turn.Discarded)
	}
	if !cardsContain(gs.Seats[0].Exile, m) || cardsContain(gs.Seats[0].Graveyard, m) {
		t.Fatalf("madness discard must replace graveyard with EXILE; exile=%v grave=%v",
			cardsContain(gs.Seats[0].Exile, m), cardsContain(gs.Seats[0].Graveyard, m))
	}
	if !HasOpenMadnessWindow(gs, 0, m) {
		t.Fatalf("madness discard must open a cast window")
	}
	// card_discarded must have fired (madness_exile event is the observable proxy).
	if countEvents(gs, "madness_exile") != 1 {
		t.Fatalf("madness_exile event should be logged once")
	}
}

// (b) declining the window puts the card into the graveyard from exile.
func TestMadness_DeclineGoesToGraveyard(t *testing.T) {
	gs := madGame()
	m := madCard(0, 2, "Fiery Temper")
	gs.Seats[0].Hand = []*Card{m}
	DiscardCard(gs, m, 0)

	ResolveMadnessWindow(gs, -1)

	if !cardsContain(gs.Seats[0].Graveyard, m) || cardsContain(gs.Seats[0].Exile, m) {
		t.Fatalf("declined madness card should be in the graveyard, not exile")
	}
	if HasOpenMadnessWindow(gs, 0, m) {
		t.Fatalf("window should be closed after resolution")
	}
}

// (e) the madness cost is an alternative cost paid when casting from the window.
func TestMadness_CastForMadnessCost(t *testing.T) {
	gs := madGame()
	m := madCard(0, 2, "Fiery Temper")
	gs.Seats[0].Hand = []*Card{m}
	DiscardCard(gs, m, 0)
	gs.Seats[0].ManaPool = 5

	if _, err := CastWithMadness(gs, 0, m, -1); err != nil {
		t.Fatalf("CastWithMadness should succeed with 5 mana for a cost-2 madness card: %v", err)
	}
	if gs.Seats[0].ManaPool != 3 {
		t.Fatalf("madness cost 2 should be paid (5→3); pool=%d", gs.Seats[0].ManaPool)
	}
	if cardsContain(gs.Seats[0].Exile, m) {
		t.Fatalf("cast madness card must leave exile")
	}
	if HasOpenMadnessWindow(gs, 0, m) {
		t.Fatalf("casting consumes the window")
	}
}

// (f) multiple discarded madness cards each get their own window; declining all
// routes each to the graveyard.
func TestMadness_MultipleEachHandled(t *testing.T) {
	gs := madGame()
	m1 := madCard(0, 1, "Madcard A")
	m1.InstanceID = "A"
	m2 := madCard(0, 1, "Madcard B")
	m2.InstanceID = "B"
	gs.Seats[0].Hand = []*Card{m1, m2}
	DiscardCard(gs, m1, 0)
	DiscardCard(gs, m2, 0)

	if !HasOpenMadnessWindow(gs, 0, m1) || !HasOpenMadnessWindow(gs, 0, m2) {
		t.Fatalf("each madness discard opens its own window")
	}
	if n := ResolveMadnessWindow(gs, -1); n != 2 {
		t.Fatalf("both declined windows should route (got %d)", n)
	}
	if !cardsContain(gs.Seats[0].Graveyard, m1) || !cardsContain(gs.Seats[0].Graveyard, m2) {
		t.Fatalf("both declined madness cards should be in the graveyard")
	}
}
