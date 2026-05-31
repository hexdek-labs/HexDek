package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestZedruu_UpkeepGainsXLifeAndDrawsX(t *testing.T) {
	gs := newGame(t, 2)
	zedruu := addPerm(gs, 0, "Zedruu the Greathearted", "creature")
	// Two permanents seat 0 owns but seat 1 controls (Zedruu's gift target).
	gifted1 := addPerm(gs, 1, "Howling Mine", "artifact")
	gifted1.Owner = 0
	gifted2 := addPerm(gs, 1, "Steel Golem", "artifact", "creature")
	gifted2.Owner = 0
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Card A", 0, "creature"),
		newCardWithTypes("Card B", 0, "creature"),
		newCardWithTypes("Card C", 0, "creature"),
	}
	preLife := gs.Seats[0].Life

	zedruuUpkeep(gs, zedruu, map[string]interface{}{"active_seat": 0})

	if gs.Seats[0].Life != preLife+2 {
		t.Errorf("expected +2 life (X=2 gifts), got %d → %d", preLife, gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected 2 cards drawn (X=2), got %d", len(gs.Seats[0].Hand))
	}
}

func TestZedruu_UpkeepNoopWhenNoGifts(t *testing.T) {
	gs := newGame(t, 2)
	zedruu := addPerm(gs, 0, "Zedruu the Greathearted", "creature")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("X", 0, "creature")}
	preLife := gs.Seats[0].Life
	zedruuUpkeep(gs, zedruu, map[string]interface{}{"active_seat": 0})
	if gs.Seats[0].Life != preLife {
		t.Errorf("expected no life change, got %d → %d", preLife, gs.Seats[0].Life)
	}
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("expected no draws, got %d cards", len(gs.Seats[0].Hand))
	}
}

func TestZedruu_UpkeepNoopOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	zedruu := addPerm(gs, 0, "Zedruu the Greathearted", "creature")
	gifted := addPerm(gs, 1, "Howling Mine", "artifact")
	gifted.Owner = 0
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("X", 0, "creature")}
	preLife := gs.Seats[0].Life
	zedruuUpkeep(gs, zedruu, map[string]interface{}{"active_seat": 1})
	if gs.Seats[0].Life != preLife || len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Zedruu must not fire on opponent's upkeep")
	}
}

func TestZedruu_OnlyCountsOpponentControlledGifts(t *testing.T) {
	gs := newGame(t, 2)
	zedruu := addPerm(gs, 0, "Zedruu the Greathearted", "creature")
	// Owned by seat 0 AND controlled by seat 0 — doesn't count.
	homePerm := addPerm(gs, 0, "Sol Ring", "artifact")
	homePerm.Owner = 0
	preLife := gs.Seats[0].Life
	zedruuUpkeep(gs, zedruu, map[string]interface{}{"active_seat": 0})
	if gs.Seats[0].Life != preLife {
		t.Errorf("permanents Zedruu's controller still controls must not count")
	}
}

func TestZedruu_ActivatedGiftEmitsPartial(t *testing.T) {
	gs := newGame(t, 2)
	zedruu := addPerm(gs, 0, "Zedruu the Greathearted", "creature")
	zedruuActivate(gs, zedruu, 0, nil)
	found := false
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_partial" {
			if d, ok := ev.Details["slug"].(string); ok && d == "zedruu_give_permanent" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected per_card_partial for the deferred gift control-change")
	}
}
