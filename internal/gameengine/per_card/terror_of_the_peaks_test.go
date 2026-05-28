package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// terror_of_the_peaks_test.go — pins the r60 Terror of the Peaks
// per_card handler that exercises the unified WardCost primitive's
// Damage variant via ApplyWardCostDamage.

func TestTerrorOfThePeaks_CreatureETB_DealsPowerToLowestToughnessOpp(t *testing.T) {
	gs := newGame(t, 2)
	terror := addPerm(gs, 0, "Terror of the Peaks", "creature", "legendary", "dragon")
	terror.Card.BasePower = 5
	terror.Card.BaseToughness = 4

	// Two opposing creatures — the 1/1 should soak the damage (lowest toughness).
	wimp := addPerm(gs, 1, "Mogg Fanatic", "creature", "goblin")
	wimp.Card.BasePower = 1
	wimp.Card.BaseToughness = 2

	smaller := addPerm(gs, 1, "Memnite", "creature", "construct")
	smaller.Card.BasePower = 1
	smaller.Card.BaseToughness = 1

	// Entering creature: 4-power Ball Lightning shape under seat 0.
	bolt := addPerm(gs, 0, "Lightning Elemental", "creature", "elemental")
	bolt.Card.BasePower = 4
	bolt.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            bolt,
	})

	if smaller.MarkedDamage != 4 {
		t.Errorf("expected the 1/1 Memnite to soak all 4 damage (lowest toughness), got %d (wimp=%d)",
			smaller.MarkedDamage, wimp.MarkedDamage)
	}
}

func TestTerrorOfThePeaks_NoCreatures_GoesToFace(t *testing.T) {
	gs := newGame(t, 2)
	terror := addPerm(gs, 0, "Terror of the Peaks", "creature", "legendary", "dragon")
	terror.Card.BasePower = 5
	terror.Card.BaseToughness = 4
	gs.Seats[1].Life = 40

	// No opposing creatures — damage routes to opponent face.
	bolt := addPerm(gs, 0, "Lightning Elemental", "creature", "elemental")
	bolt.Card.BasePower = 4
	bolt.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            bolt,
	})

	if gs.Seats[1].Life != 36 {
		t.Errorf("expected seat 1 life = 36 (40 - 4), got %d", gs.Seats[1].Life)
	}
}

func TestTerrorOfThePeaks_SelfETB_NoSelfDamage(t *testing.T) {
	// When Terror itself enters, the "another creature" gate suppresses
	// the trigger — no damage anywhere.
	gs := newGame(t, 2)
	terror := addPerm(gs, 0, "Terror of the Peaks", "creature", "legendary", "dragon")
	terror.Card.BasePower = 5
	terror.Card.BaseToughness = 4
	gs.Seats[1].Life = 40

	opp := addPerm(gs, 1, "Memnite", "creature", "construct")
	opp.Card.BasePower = 1
	opp.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 0,
		"perm":            terror, // entering = Terror itself
	})

	if opp.MarkedDamage != 0 {
		t.Errorf("self-ETB should not damage opponents, got %d marked", opp.MarkedDamage)
	}
	if gs.Seats[1].Life != 40 {
		t.Errorf("self-ETB should not damage face, got life %d", gs.Seats[1].Life)
	}
}

func TestTerrorOfThePeaks_OpponentCreatureETB_NoTrigger(t *testing.T) {
	// "creature you control enters" — opponent's ETB must not fire Terror.
	gs := newGame(t, 2)
	addPerm(gs, 0, "Terror of the Peaks", "creature", "legendary", "dragon").Card.BasePower = 5
	gs.Seats[0].Life = 40

	oppCreature := addPerm(gs, 1, "Lightning Elemental", "creature", "elemental")
	oppCreature.Card.BasePower = 4
	oppCreature.Card.BaseToughness = 1

	// Filler creature on seat 0 so the lowest-toughness pick has somewhere
	// to land if the gate fails.
	wimp := addPerm(gs, 0, "Memnite", "creature", "construct")
	wimp.Card.BasePower = 1
	wimp.Card.BaseToughness = 1

	gameengine.FireCardTrigger(gs, "nonland_permanent_etb", map[string]interface{}{
		"controller_seat": 1,
		"perm":            oppCreature,
	})

	if gs.Seats[0].Life != 40 {
		t.Errorf("opponent-controlled ETB should not fire Terror, but seat 0 life dropped to %d", gs.Seats[0].Life)
	}
	if wimp.MarkedDamage != 0 {
		t.Errorf("opponent ETB must not damage your own creature, got %d", wimp.MarkedDamage)
	}
}

// Charging War Boar — Ward—Pay 3 life — wired via SetWardCost in the
// per_card layer. Pin the ETB wiring flows the WardCost correctly.
func TestChargingWarBoar_ETB_StampsPayLifeWard(t *testing.T) {
	gs := newGame(t, 2)
	boar := addPerm(gs, 0, "Charging War Boar", "creature")
	boar.Card.BasePower = 4
	boar.Card.BaseToughness = 4

	// Manually drive ETB since addPerm doesn't fire it.
	chargingWarBoarETB(gs, boar)

	cost := gameengine.GetWardCost(boar)
	if cost.Type != gameengine.WardCostLife {
		t.Errorf("WardCost.Type = %d, want Life", cost.Type)
	}
	if cost.Amount != 3 {
		t.Errorf("WardCost.Amount = %d, want 3", cost.Amount)
	}
}
