package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// mortivore_repro_r61_test.go — repro for the 2026-06-10 spectator bug:
// "how do I destroy my own Mortivore unless there are zero creatures in all
// graveyards?" — Mortivore (*/* = creature cards in all graveyards) read 0/0
// and died despite creatures sitting in graveyards.

// gyCreature builds a creature-typed card for a graveyard slot.
func gyCreature(name string) *gameengine.Card {
	return &gameengine.Card{Name: name, Types: []string{"creature"}}
}

func TestMortivore_CDA_CountsCreatureCardsInAllYards(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	// 3 creatures in seat 0's yard, 2 in seat 1's yard = 5 total.
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		gyCreature("Grizzly Bears"), gyCreature("Llanowar Elves"), gyCreature("Birds"),
	)
	gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard,
		gyCreature("Goblin"), gyCreature("Zombie"),
	)
	gameengine.InvokeETBHook(gs, morti)

	chars := gameengine.GetEffectiveCharacteristics(gs, morti)
	if chars.Power != 5 || chars.Toughness != 5 {
		t.Errorf("Mortivore should be 5/5 (5 creature cards in yards), got %d/%d", chars.Power, chars.Toughness)
	}
}

func TestMortivore_CDA_IgnoresNoncreatureCards(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		gyCreature("Grizzly Bears"),                                   // counts
		&gameengine.Card{Name: "Lightning Bolt", Types: []string{"instant"}},  // no
		&gameengine.Card{Name: "Forest", Types: []string{"land"}},     // no
		&gameengine.Card{Name: "Sol Ring", Types: []string{"artifact"}}, // no
	)
	gameengine.InvokeETBHook(gs, morti)

	chars := gameengine.GetEffectiveCharacteristics(gs, morti)
	if chars.Power != 1 || chars.Toughness != 1 {
		t.Errorf("Mortivore should be 1/1 (only 1 creature card in yards), got %d/%d", chars.Power, chars.Toughness)
	}
}

// The actual bug surface: with creatures in graveyards Mortivore must NOT be
// destroyed by the §704.5f "toughness 0 or less" state-based action.
func TestMortivore_SurvivesSBA_WithCreaturesInYards(t *testing.T) {
	gs := newGame(t, 2)
	morti := addPerm(gs, 0, "Mortivore", "creature")
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
		gyCreature("A"), gyCreature("B"), gyCreature("C"),
	)
	gameengine.InvokeETBHook(gs, morti)

	gameengine.StateBasedActions(gs)

	// Mortivore should still be on seat 0's battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Card != nil && p.Card.Name == "Mortivore" {
			found = true
		}
	}
	if !found {
		t.Errorf("Mortivore was destroyed by SBA despite 3 creatures in the graveyard (read as 0/0)")
	}
}
