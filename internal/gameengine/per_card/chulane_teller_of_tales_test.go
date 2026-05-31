package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestChulane_DrawsAndPlaysLandOnOwnCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	chulane := addPerm(gs, 0, "Chulane, Teller of Tales", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Filler", 0, "creature"),
	}
	gs.Seats[0].Hand = []*gameengine.Card{
		newCardWithTypes("Forest", 0, "land"),
	}

	chulaneCreatureCast(gs, chulane, map[string]interface{}{
		"caster_seat": 0,
		"spell_name":  "Llanowar Elves",
	})

	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].DisplayName() != "Filler" {
		t.Errorf("expected drawn Filler in hand and Forest removed, got %v", handNames(gs.Seats[0].Hand))
	}
	foundForest := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Forest" {
			foundForest = true
		}
	}
	if !foundForest {
		t.Errorf("expected Forest played onto battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
}

func TestChulane_DoesNotFireOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	chulane := addPerm(gs, 0, "Chulane, Teller of Tales", "creature")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Filler", 0, "creature")}
	gs.Seats[0].Hand = []*gameengine.Card{newCardWithTypes("Forest", 0, "land")}

	chulaneCreatureCast(gs, chulane, map[string]interface{}{"caster_seat": 1})

	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].DisplayName() != "Forest" {
		t.Errorf("Chulane must not fire on opponent's cast; hand should be unchanged, got %v", handNames(gs.Seats[0].Hand))
	}
}

func TestChulane_DrawsEvenWhenNoLandInHand(t *testing.T) {
	gs := newGame(t, 2)
	chulane := addPerm(gs, 0, "Chulane, Teller of Tales", "creature")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Drew Card", 0, "creature")}
	// Hand has no land.
	gs.Seats[0].Hand = []*gameengine.Card{newCardWithTypes("Counterspell", 0, "instant")}

	chulaneCreatureCast(gs, chulane, map[string]interface{}{"caster_seat": 0})

	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected draw to happen even without land in hand, got %d cards", len(gs.Seats[0].Hand))
	}
}

func TestChulane_NoopOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	chulane := addPerm(gs, 0, "Chulane, Teller of Tales", "creature")
	chulaneCreatureCast(gs, chulane, map[string]interface{}{"caster_seat": 0})
	// drawOne should set AttemptedEmptyDraw without panicking.
	if !gs.Seats[0].AttemptedEmptyDraw {
		t.Errorf("expected AttemptedEmptyDraw on empty library")
	}
}
