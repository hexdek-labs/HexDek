package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

func TestKarametra_RampsForestOnOwnCreatureCast(t *testing.T) {
	gs := newGame(t, 2)
	karametra := addPerm(gs, 0, "Karametra, God of Harvests", "creature", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Mountain", 0, "land"),
		newCardWithTypes("Forest", 0, "land"),
		newCardWithTypes("Plains", 0, "land"),
	}

	karametraCreatureCast(gs, karametra, map[string]interface{}{"caster_seat": 0})

	foundLand := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Forest" {
			foundLand = true
			if !p.Tapped {
				t.Errorf("Karametra puts the land onto the battlefield TAPPED, got untapped")
			}
		}
	}
	if !foundLand {
		t.Errorf("expected Forest on battlefield, got %v", permNames(gs.Seats[0].Battlefield))
	}
}

func TestKarametra_PrefersFirstForestOrPlainsOverMountain(t *testing.T) {
	gs := newGame(t, 2)
	karametra := addPerm(gs, 0, "Karametra, God of Harvests", "creature", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{
		newCardWithTypes("Mountain", 0, "land"),
		newCardWithTypes("Plains", 0, "land"),
	}
	karametraCreatureCast(gs, karametra, map[string]interface{}{"caster_seat": 0})

	foundPlains := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Plains" {
			foundPlains = true
		}
	}
	if !foundPlains {
		t.Errorf("expected Plains to be selected (Mountain ignored), got %v", permNames(gs.Seats[0].Battlefield))
	}
}

func TestKarametra_DoesNotFireOnOpponentCast(t *testing.T) {
	gs := newGame(t, 2)
	karametra := addPerm(gs, 0, "Karametra, God of Harvests", "creature", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Forest", 0, "land")}
	karametraCreatureCast(gs, karametra, map[string]interface{}{"caster_seat": 1})
	if len(gs.Seats[0].Library) != 1 {
		t.Errorf("Karametra must not ramp on opponent's cast, library should be unchanged")
	}
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.DisplayName() == "Forest" {
			t.Errorf("Karametra fired on opponent's cast — should have no-op'd")
		}
	}
}

func TestKarametra_NoLandFoundEmitsFail(t *testing.T) {
	gs := newGame(t, 2)
	karametra := addPerm(gs, 0, "Karametra, God of Harvests", "creature", "enchantment")
	gs.Seats[0].Library = []*gameengine.Card{newCardWithTypes("Mountain", 0, "land")}
	karametraCreatureCast(gs, karametra, map[string]interface{}{"caster_seat": 0})
	failCount := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "per_card_failed" {
			if d, ok := ev.Details["slug"].(string); ok && d == "karametra_ramp_forest_or_plains" {
				failCount++
			}
		}
	}
	if failCount != 1 {
		t.Errorf("expected 1 per_card_failed event for no_forest_or_plains, got %d", failCount)
	}
}
