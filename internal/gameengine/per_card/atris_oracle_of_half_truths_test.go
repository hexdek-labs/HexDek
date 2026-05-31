package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestAtris_ETBSplitsAndPicksHigherAggregateCMCPileToHand(t *testing.T) {
	gs := newGame(t, 2)
	atris := addPerm(gs, 0, "Atris, Oracle of Half-Truths", "creature")
	// Top 3: [cmc:1, cmc:2, cmc:6]. Opponent splits as {6} vs {1,2}.
	// Aggregate cmc: 6 vs 3. Atris controller picks 6 to hand.
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("A", 0, "creature", "cmc:1"),
		newCardWithTypes("B", 0, "creature", "cmc:2"),
		newCardWithTypes("C", 0, "creature", "cmc:6"),
	}
	atrisETB(gs, atris)
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].DisplayName() != "C" {
		t.Errorf("expected C in hand (single high-CMC pile), got %v", handNames(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 cards in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
}

func TestAtris_ETBPicksTwoCardPileWhenAggregateHigher(t *testing.T) {
	gs := newGame(t, 2)
	atris := addPerm(gs, 0, "Atris, Oracle of Half-Truths", "creature")
	// Top 3: [cmc:5, cmc:4, cmc:3]. Opponent splits {5} alone vs {4,3}.
	// Aggregate cmc: 5 vs 7. Pick the 2-card pile to hand.
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("A", 0, "creature", "cmc:5"),
		newCardWithTypes("B", 0, "creature", "cmc:4"),
		newCardWithTypes("C", 0, "creature", "cmc:3"),
	}
	atrisETB(gs, atris)
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards in hand (higher aggregate pile), got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected 1 card (the highest-CMC singleton) in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
}

func TestAtris_HandlesShortLibrary(t *testing.T) {
	gs := newGame(t, 2)
	atris := addPerm(gs, 0, "Atris, Oracle of Half-Truths", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Only", 0, "creature", "cmc:3"),
	}
	atrisETB(gs, atris)
	if len(gs.Seats[0].Hand)+len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("expected the single library card to land in hand or graveyard")
	}
}

func TestAtris_NoopOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	atris := addPerm(gs, 0, "Atris, Oracle of Half-Truths", "creature")
	atrisETB(gs, atris)
	if len(gs.Seats[0].Hand) != 0 || len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected no zone changes on empty library")
	}
}
