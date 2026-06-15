package gameengine

import (
	"math/rand"
	"testing"
)

// These pin CR §118.9 / §601.2f for the graveyard/exile alternative-cost cast
// paths: a Thalia/Sphere tax raises the alternative cost, a reducer lowers it.
// Before r63 these paths paid the raw printed alternative cost, bypassing the
// cost-modifier pipeline entirely.

// --- Flashback (graveyard) -------------------------------------------------

func TestCastFlashback_TaxRaisesAltCost(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5 // exactly the raw flashback cost
	addPerm(gs, 0, "Sphere of Resistance", []string{"artifact"})

	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	// Raw flashback cost 5, taxed to 6 → 5 mana is insufficient.
	if _, err := CastFlashback(gs, 0, card, 5); err == nil {
		t.Fatal("flashback with Sphere of Resistance: 5 mana must be insufficient for a taxed cost of 6")
	}
}

func TestCastFlashback_ReducerLowersAltCost(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	gs.Seats[0].ManaPool = 5
	addPerm(gs, 0, "Helm of Awakening", []string{"artifact"})

	card := newFlashbackCard("Past in Flames", 0, 4, "{4}{R}")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)

	// Raw flashback cost 5, reduced to 4 → 5 mana, 1 left.
	if _, err := CastFlashback(gs, 0, card, 5); err != nil {
		t.Fatalf("flashback with Helm of Awakening should succeed at reduced cost: %v", err)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("reduced flashback cost should leave 1 mana (5 - 4), got %d", gs.Seats[0].ManaPool)
	}
}

// --- Madness (exile) -------------------------------------------------------

func TestCastWithMadness_TaxRaisesAltCost(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.Active = 0
	gs.Seats[0].ManaPool = 3
	addPerm(gs, 0, "Sphere of Resistance", []string{"artifact"})

	card := newMadnessCard("Fiery Temper", 0, 4, "{R}")
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, card)
	// Discard opens the madness window and exiles the card.
	DiscardCard(gs, card, 0)

	// Raw madness cost 1, taxed to 2 → 3 mana, 1 left.
	if _, err := CastWithMadness(gs, 0, card, 1); err != nil {
		t.Fatalf("CastWithMadness should succeed at taxed cost: %v", err)
	}
	if gs.Seats[0].ManaPool != 1 {
		t.Fatalf("taxed madness cost should leave 1 mana (3 - 2), got %d", gs.Seats[0].ManaPool)
	}
}
