package per_card

import (
	"testing"
)

func TestSaskia_ETBChoosesHighestLifeOpponent(t *testing.T) {
	gs := newGame(t, 4)
	gs.Seats[1].Life = 30
	gs.Seats[2].Life = 50 // highest
	gs.Seats[3].Life = 20
	saskia := addPerm(gs, 0, "Saskia the Unyielding", "creature")
	saskiaETB(gs, saskia)
	if saskia.Flags["saskia_target_seat"] != 3 { // seat 2 stored as 2+1=3
		t.Errorf("expected chosen_player = seat 2 (highest life), stored as 3 (+1), got %d",
			saskia.Flags["saskia_target_seat"])
	}
}

func TestSaskia_RedirectsCombatDamageToChosenPlayer(t *testing.T) {
	gs := newGame(t, 4)
	saskia := addPerm(gs, 0, "Saskia the Unyielding", "creature")
	saskia.Flags["saskia_target_seat"] = 3 // chosen = seat 2 (+1 offset)
	preLife := gs.Seats[2].Life

	saskiaCombatDamage(gs, saskia, map[string]interface{}{
		"source_seat":   0,
		"source_card":   "Some Creature",
		"defender_seat": 1,
		"amount":        5,
	})

	if gs.Seats[2].Life != preLife-5 {
		t.Errorf("expected chosen seat 2 to take 5 redirected damage, life %d → %d (preLife=%d)",
			preLife, gs.Seats[2].Life, preLife)
	}
}

func TestSaskia_DoesNotRedirectOnOpponentDamage(t *testing.T) {
	gs := newGame(t, 4)
	saskia := addPerm(gs, 0, "Saskia the Unyielding", "creature")
	saskia.Flags["saskia_target_seat"] = 3
	preLife := gs.Seats[2].Life
	saskiaCombatDamage(gs, saskia, map[string]interface{}{
		"source_seat":   1, // opponent's creature attacking
		"defender_seat": 0,
		"amount":        4,
	})
	if gs.Seats[2].Life != preLife {
		t.Errorf("Saskia must only redirect from her controller's creatures")
	}
}

func TestSaskia_NoDoubleUpWhenDefenderIsChosenPlayer(t *testing.T) {
	gs := newGame(t, 4)
	saskia := addPerm(gs, 0, "Saskia the Unyielding", "creature")
	saskia.Flags["saskia_target_seat"] = 3 // chosen = seat 2
	preLife := gs.Seats[2].Life
	saskiaCombatDamage(gs, saskia, map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 2, // already attacking the chosen player
		"amount":        3,
	})
	if gs.Seats[2].Life != preLife {
		t.Errorf("must not double-up when defender already equals chosen player; got life %d → %d",
			preLife, gs.Seats[2].Life)
	}
}

func TestSaskia_NoFireWhenNoPlayerChosen(t *testing.T) {
	gs := newGame(t, 4)
	saskia := addPerm(gs, 0, "Saskia the Unyielding", "creature")
	// saskia_target_seat unset
	preLife := gs.Seats[2].Life
	saskiaCombatDamage(gs, saskia, map[string]interface{}{
		"source_seat":   0,
		"defender_seat": 1,
		"amount":        4,
	})
	if gs.Seats[2].Life != preLife {
		t.Errorf("no redirect should fire without saskia_target_seat set")
	}
}
