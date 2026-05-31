package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestNiambi_ETBBouncesAnotherCreatureAndGainsLife(t *testing.T) {
	gs := newGame(t, 2)
	niambi := addPerm(gs, 0, "Niambi, Esteemed Speaker", "creature", "human")
	target := addPerm(gs, 0, "Mulldrifter", "creature")
	target.Card.Types = append(target.Card.Types, "cmc:5")
	preLife := gs.Seats[0].Life

	niambiETB(gs, niambi)

	stillOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == target {
			stillOnBF = true
		}
	}
	if stillOnBF {
		t.Errorf("expected target bounced off battlefield")
	}
	if gs.Seats[0].Life != preLife+5 {
		t.Errorf("expected +5 life (target CMC = 5), got %d → %d", preLife, gs.Seats[0].Life)
	}
}

func TestNiambi_ETBPicksHighestCMCWhenMultipleCreatures(t *testing.T) {
	gs := newGame(t, 2)
	niambi := addPerm(gs, 0, "Niambi, Esteemed Speaker", "creature", "human")
	small := addPerm(gs, 0, "Small", "creature")
	small.Card.Types = append(small.Card.Types, "cmc:1")
	big := addPerm(gs, 0, "Big", "creature")
	big.Card.Types = append(big.Card.Types, "cmc:6")
	preLife := gs.Seats[0].Life

	niambiETB(gs, niambi)

	if gs.Seats[0].Life != preLife+6 {
		t.Errorf("expected highest-CMC target picked (+6 life), got %d → %d", preLife, gs.Seats[0].Life)
	}
}

func TestNiambi_ETBNoopWithNoOtherCreature(t *testing.T) {
	gs := newGame(t, 2)
	niambi := addPerm(gs, 0, "Niambi, Esteemed Speaker", "creature", "human")
	preLife := gs.Seats[0].Life
	niambiETB(gs, niambi)
	if gs.Seats[0].Life != preLife {
		t.Errorf("no life gain expected when no other creature exists")
	}
}

func TestNiambi_ActivatedDrawsTwo(t *testing.T) {
	gs := newGame(t, 2)
	niambi := addPerm(gs, 0, "Niambi, Esteemed Speaker", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("A", 0, "creature"),
		newCardWithTypes("B", 0, "creature"),
		newCardWithTypes("C", 0, "creature"),
	}
	niambiActivate(gs, niambi, 0, nil)
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards drawn, got %d", len(gs.Seats[0].Hand))
	}
}
