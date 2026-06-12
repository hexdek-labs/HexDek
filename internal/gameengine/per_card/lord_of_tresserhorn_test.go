package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestLordOfTresserhorn_ETBLifeLossAndOppDrawAndSacrifices(t *testing.T) {
	gs := newGame(t, 2)
	lord := addPerm(gs, 0, "Lord of Tresserhorn", "creature", "zombie")
	addPerm(gs, 0, "Sac Fodder A", "creature")
	addPerm(gs, 0, "Sac Fodder B", "creature")
	gs.Seats[1].Library = []*gameengine.Card{
		newCardWithTypes("Drew A", 1, "creature"),
		newCardWithTypes("Drew B", 1, "creature"),
	}
	preLife := gs.Seats[0].Life

	lordOfTresserhornETB(gs, lord)

	if gs.Seats[0].Life != preLife-2 {
		t.Errorf("expected -2 life, got %d → %d", preLife, gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Graveyard) != 2 {
		t.Errorf("expected 2 sacrificed creatures in graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[1].Hand) != 2 {
		t.Errorf("expected target opponent drew 2, got %d cards in hand", len(gs.Seats[1].Hand))
	}
}

func TestLordOfTresserhorn_PrefersLowestPowerFodder(t *testing.T) {
	gs := newGame(t, 2)
	lord := addPerm(gs, 0, "Lord of Tresserhorn", "creature", "zombie")
	small := addPerm(gs, 0, "Small", "creature")
	small.Card.BasePower = 1
	big := addPerm(gs, 0, "Big", "creature")
	big.Card.BasePower = 7
	mid := addPerm(gs, 0, "Mid", "creature")
	mid.Card.BasePower = 3
	gs.Seats[1].Library = []*gameengine.Card{newCardWithTypes("X", 1, "creature")}
	lordOfTresserhornETB(gs, lord)
	// Should have sacrificed Small + Mid (the two lowest), not Big.
	for _, c := range gs.Seats[0].Graveyard {
		if c != nil && c.DisplayName() == "Big" {
			t.Errorf("Big should not be sacrificed when smaller fodder available")
		}
	}
}

func TestLordOfTresserhorn_HandlesShortLibraryForOppDraw(t *testing.T) {
	gs := newGame(t, 2)
	lord := addPerm(gs, 0, "Lord of Tresserhorn", "creature", "zombie")
	addPerm(gs, 0, "X", "creature")
	addPerm(gs, 0, "Y", "creature")
	// Opp library empty — should not crash, and the empty-draw attempt
	// must be registered. Since the r62 CheckEnd pre-sweep (CR §704.3:
	// player-loss SBAs apply before game-over evaluation), the handler's
	// own CheckEnd call may already have consumed AttemptedEmptyDraw and
	// marked the seat Lost per §704.5b — either state satisfies the
	// test's intent.
	lordOfTresserhornETB(gs, lord)
	if !gs.Seats[1].AttemptedEmptyDraw && !gs.Seats[1].Lost {
		t.Errorf("expected AttemptedEmptyDraw flag or §704.5b loss on opponent")
	}
}
