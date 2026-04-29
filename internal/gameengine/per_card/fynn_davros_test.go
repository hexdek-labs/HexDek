package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// ============================================================================
// Fynn, the Fangbearer
// ============================================================================

func TestFynn_DeathtouchCreatureGivesPoisonOnCombatDamage(t *testing.T) {
	gs := newGame(t, 2)
	// Place Fynn on the battlefield.
	_ = addPerm(gs, 0, "Fynn, the Fangbearer", "creature", "legendary")

	// Place a creature with deathtouch controlled by seat 0.
	dtCreature := addPerm(gs, 0, "Typhoid Rats", "creature")
	dtCreature.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "deathtouch"},
		},
	}

	// Fire the combat_damage_player trigger (simulating the deathtouch
	// creature dealing combat damage to player 1).
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":  0,
		"source_card":  "Typhoid Rats",
		"defender_seat": 1,
		"amount":       1,
	})

	// Player 1 should have 2 poison counters from Fynn's trigger.
	if gs.Seats[1].PoisonCounters != 2 {
		t.Errorf("expected 2 poison counters from Fynn trigger, got %d", gs.Seats[1].PoisonCounters)
	}
}

func TestFynn_NonDeathtouchCreatureNoPoisin(t *testing.T) {
	gs := newGame(t, 2)
	_ = addPerm(gs, 0, "Fynn, the Fangbearer", "creature", "legendary")

	// Creature without deathtouch.
	_ = addPerm(gs, 0, "Grizzly Bears", "creature")

	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":  0,
		"source_card":  "Grizzly Bears",
		"defender_seat": 1,
		"amount":       2,
	})

	// No poison counters from Fynn (no deathtouch).
	if gs.Seats[1].PoisonCounters != 0 {
		t.Errorf("expected 0 poison counters without deathtouch, got %d", gs.Seats[1].PoisonCounters)
	}
}

func TestFynn_OpponentCreatureNoPoisin(t *testing.T) {
	gs := newGame(t, 2)
	_ = addPerm(gs, 0, "Fynn, the Fangbearer", "creature", "legendary")

	// Opponent's creature with deathtouch.
	dtCreature := addPerm(gs, 1, "Gifted Aetherborn", "creature")
	dtCreature.Card.AST = &gameast.CardAST{
		Abilities: []gameast.Ability{
			&gameast.Keyword{Name: "deathtouch"},
		},
	}

	// The source is controlled by seat 1 (opponent of Fynn's controller).
	gameengine.FireCardTrigger(gs, "combat_damage_player", map[string]interface{}{
		"source_seat":  1,
		"source_card":  "Gifted Aetherborn",
		"defender_seat": 0,
		"amount":       2,
	})

	// No poison from Fynn because it's not controlled by Fynn's controller.
	if gs.Seats[0].PoisonCounters != 0 {
		t.Errorf("expected 0 poison counters from opponent's creature, got %d", gs.Seats[0].PoisonCounters)
	}
}

// ============================================================================
// Davros, Dalek Creator
// ============================================================================

func TestDavros_EliminationGivesRadCounters(t *testing.T) {
	gs := newGame(t, 4)
	// Davros on seat 0's battlefield.
	_ = addPerm(gs, 0, "Davros, Dalek Creator", "creature", "legendary")

	// Eliminate seat 2.
	gs.Seats[2].Lost = true
	gs.Seats[2].LossReason = "damage"
	gameengine.FireCardTrigger(gs, "seat_eliminated", map[string]interface{}{
		"eliminated_seat": 2,
		"reason":          "damage",
	})

	// Each opponent of Davros's controller (seats 1, 2, 3) should get
	// 3 rad counters, EXCEPT seat 2 which is already lost.
	if gs.Seats[1].Flags == nil || gs.Seats[1].Flags["rad_counters"] != 3 {
		t.Errorf("seat 1 expected 3 rad counters, got %d", gs.Seats[1].Flags["rad_counters"])
	}
	// Seat 2 is lost, should be skipped.
	if gs.Seats[2].Flags != nil && gs.Seats[2].Flags["rad_counters"] != 0 {
		t.Errorf("seat 2 (eliminated) should not get rad counters, got %d", gs.Seats[2].Flags["rad_counters"])
	}
	if gs.Seats[3].Flags == nil || gs.Seats[3].Flags["rad_counters"] != 3 {
		t.Errorf("seat 3 expected 3 rad counters, got %d", gs.Seats[3].Flags["rad_counters"])
	}
	// Davros's controller (seat 0) should NOT get rad counters.
	if gs.Seats[0].Flags != nil && gs.Seats[0].Flags["rad_counters"] != 0 {
		t.Errorf("seat 0 (Davros controller) should not get rad counters, got %d", gs.Seats[0].Flags["rad_counters"])
	}
}
