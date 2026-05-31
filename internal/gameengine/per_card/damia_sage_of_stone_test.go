package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestDamia_ETBSetsSkipDrawFlag(t *testing.T) {
	gs := newGame(t, 2)
	damia := addPerm(gs, 0, "Damia, Sage of Stone", "creature")
	damiaETB(gs, damia)
	if gs.Flags["damia_skip_draw_seat_0"] == 0 {
		t.Errorf("expected damia_skip_draw_seat_0 flag set, got 0")
	}
}

func TestDamia_UpkeepRefillsHandToSeven(t *testing.T) {
	gs := newGame(t, 2)
	damia := addPerm(gs, 0, "Damia, Sage of Stone", "creature")
	gs.Seats[0].Hand = []*gameengine.Card{
		newCardWithTypes("A", 0, "creature"),
		newCardWithTypes("B", 0, "creature"),
	}
	for i := 0; i < 10; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, newCardWithTypes("L", 0, "creature"))
	}
	damiaUpkeep(gs, damia, map[string]interface{}{"active_seat": 0})
	if len(gs.Seats[0].Hand) != 7 {
		t.Errorf("expected hand refilled to 7, got %d", len(gs.Seats[0].Hand))
	}
}

func TestDamia_UpkeepNoopWhenHandAlreadyAtSeven(t *testing.T) {
	gs := newGame(t, 2)
	damia := addPerm(gs, 0, "Damia, Sage of Stone", "creature")
	for i := 0; i < 7; i++ {
		gs.Seats[0].Hand = append(gs.Seats[0].Hand, newCardWithTypes("X", 0, "creature"))
	}
	for i := 0; i < 5; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, newCardWithTypes("L", 0, "creature"))
	}
	damiaUpkeep(gs, damia, map[string]interface{}{"active_seat": 0})
	if len(gs.Seats[0].Hand) != 7 {
		t.Errorf("hand should remain at 7, got %d", len(gs.Seats[0].Hand))
	}
	if len(gs.Seats[0].Library) != 5 {
		t.Errorf("library should be unchanged at 5, got %d", len(gs.Seats[0].Library))
	}
}

func TestDamia_UpkeepNoopOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	damia := addPerm(gs, 0, "Damia, Sage of Stone", "creature")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("X", 0, "creature")}
	damiaUpkeep(gs, damia, map[string]interface{}{"active_seat": 1})
	if len(gs.Seats[0].Hand) != 0 {
		t.Errorf("Damia must not fire on opponent's upkeep")
	}
}

func TestDamia_UpkeepStopsOnEmptyLibrary(t *testing.T) {
	gs := newGame(t, 2)
	damia := addPerm(gs, 0, "Damia, Sage of Stone", "creature")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("X", 0, "creature"),
		newCardWithTypes("Y", 0, "creature"),
	}
	damiaUpkeep(gs, damia, map[string]interface{}{"active_seat": 0})
	if len(gs.Seats[0].Hand) != 2 {
		t.Errorf("expected to draw only 2 (library size), got %d", len(gs.Seats[0].Hand))
	}
	if !gs.Seats[0].AttemptedEmptyDraw {
		t.Errorf("expected AttemptedEmptyDraw set after exhausting library")
	}
}
