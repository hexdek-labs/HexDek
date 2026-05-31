package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestDeadbridgeChant_ETBMillsTenCards(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 20; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library,
			newCardWithTypes("Filler", 0, "creature"))
	}
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")

	deadbridgeChantETB(gs, dbc)

	if len(gs.Seats[0].Graveyard) != 10 {
		t.Errorf("expected 10 cards milled to graveyard, got %d", len(gs.Seats[0].Graveyard))
	}
	if len(gs.Seats[0].Library) != 10 {
		t.Errorf("expected 10 cards still in library, got %d", len(gs.Seats[0].Library))
	}
}

func TestDeadbridgeChant_ETBMillCappedByLibrarySize(t *testing.T) {
	gs := newGame(t, 2)
	for i := 0; i < 3; i++ {
		gs.Seats[0].Library = append(gs.Seats[0].Library, newCardWithTypes("X", 0, "creature"))
	}
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")
	deadbridgeChantETB(gs, dbc)
	if len(gs.Seats[0].Graveyard) != 3 {
		t.Errorf("mill should cap at library size, got %d", len(gs.Seats[0].Graveyard))
	}
}

func TestDeadbridgeChant_UpkeepCreatureToBattlefield(t *testing.T) {
	gs := newGame(t, 2)
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")
	gs.Seats[0].Graveyard = []*gameengine.Card{
		newCardWithTypes("Llanowar Elves", 0, "creature"),
	}
	deadbridgeChantUpkeep(gs, dbc, map[string]interface{}{"active_seat": 0})
	foundOnBF := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Llanowar Elves" {
			foundOnBF = true
		}
	}
	if !foundOnBF {
		t.Errorf("expected Llanowar Elves on battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Graveyard) != 0 {
		t.Errorf("expected graveyard emptied of the chosen card, got %d remaining", len(gs.Seats[0].Graveyard))
	}
}

func TestDeadbridgeChant_UpkeepNoncreatureToHand(t *testing.T) {
	gs := newGame(t, 2)
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")
	gs.Seats[0].Graveyard = []*gameengine.Card{newCardWithTypes("Counterspell", 0, "instant")}
	deadbridgeChantUpkeep(gs, dbc, map[string]interface{}{"active_seat": 0})
	if len(gs.Seats[0].Hand) != 1 || gs.Seats[0].Hand[0].DisplayName() != "Counterspell" {
		t.Errorf("expected Counterspell in hand, got %v", handNames(gs.Seats[0].Hand))
	}
}

func TestDeadbridgeChant_UpkeepNoopOnOpponentTurn(t *testing.T) {
	gs := newGame(t, 2)
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")
	gs.Seats[0].Graveyard = []*gameengine.Card{newCardWithTypes("Llanowar Elves", 0, "creature")}
	deadbridgeChantUpkeep(gs, dbc, map[string]interface{}{"active_seat": 1})
	if len(gs.Seats[0].Battlefield) != 1 { // just the DBC itself
		t.Errorf("expected no-op on opponent's upkeep, battlefield=%v", permNames(gs.Seats[0].Battlefield))
	}
	if len(gs.Seats[0].Graveyard) != 1 {
		t.Errorf("graveyard should be unchanged")
	}
}

func TestDeadbridgeChant_UpkeepNoopOnEmptyGraveyard(t *testing.T) {
	gs := newGame(t, 2)
	dbc := addPerm(gs, 0, "Deadbridge Chant", "enchantment")
	deadbridgeChantUpkeep(gs, dbc, map[string]interface{}{"active_seat": 0})
}

