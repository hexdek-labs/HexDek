package main

// Module 1: Keyword Combat Matrix (--keyword-matrix)
//
// Creates two 3/3 creatures, one with keyword A, one with keyword B.
// Declares A as attacker (seat 0), B as blocker (seat 1).
// Runs DealCombatDamageStep, SBAs, then checks all invariants.
// ~30 keywords x 30 = 900 combat pairs.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// combatKeywords are keywords that have meaningful combat interactions.
var combatKeywords = []string{
	"flying", "reach", "trample", "deathtouch",
	"first_strike", "double_strike", "lifelink", "vigilance",
	"menace", "indestructible", "hexproof", "shroud",
	"haste", "flash", "defender",
	"intimidate", "fear", "shadow", "skulk",
	"protection_from_red", "protection_from_black",
	"infect", "wither", "flanking", "bushido",
	"horsemanship", "banding",
	"annihilator", "afflict", "battle_cry", "myriad",
	"rampage", "provoke",
}

func runKeywordMatrix(_ *astload.Corpus, _ []*oracleCard) []failure {
	var fails []failure

	for _, kwA := range combatKeywords {
		for _, kwB := range combatKeywords {
			f := testKeywordPair(kwA, kwB)
			if f != nil {
				fails = append(fails, *f)
			}
		}
	}

	return fails
}

func testKeywordPair(kwA, kwB string) (result *failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			result = &failure{
				CardName:    fmt.Sprintf("%s_vs_%s", kwA, kwB),
				Interaction: "keyword_matrix",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			}
		}
	}()

	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "combat",
		Step:   "combat_damage",
		Flags:  map[string]int{},
	}

	// 2 seats with basic setup.
	for i := 0; i < 2; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Idx:   i,
			Flags: map[string]int{},
		}
		for j := 0; j < 5; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler %d-%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}

	// Create attacker on seat 0 with keyword A.
	atkCard := &gameengine.Card{
		Name: fmt.Sprintf("TestCreature_%s", kwA), Owner: 0,
		Types: []string{"creature"}, Colors: []string{"R"},
		BasePower: 3, BaseToughness: 3,
	}
	atkPerm := &gameengine.Permanent{
		Card:       atkCard,
		Controller: 0, Owner: 0,
		Flags: map[string]int{
			"kw:" + kwA:   1,
			"attacking":   1,
			"defender_seat_p1": 2, // defender = seat 1 (offset by 1)
		},
		Tapped: true, // attackers are tapped
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, atkPerm)

	// Create blocker on seat 1 with keyword B.
	blkCard := &gameengine.Card{
		Name: fmt.Sprintf("TestCreature_%s", kwB), Owner: 1,
		Types: []string{"creature"}, Colors: []string{"B"},
		BasePower: 3, BaseToughness: 3,
	}
	blkPerm := &gameengine.Permanent{
		Card:       blkCard,
		Controller: 1, Owner: 1,
		Flags: map[string]int{
			"kw:" + kwB: 1,
			"blocking":  1,
		},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, blkPerm)

	gs.Snapshot()

	// Build attacker/blocker map.
	attackers := []*gameengine.Permanent{atkPerm}
	blockerMap := map[*gameengine.Permanent][]*gameengine.Permanent{
		atkPerm: {blkPerm},
	}

	// Deal combat damage.
	gameengine.DealCombatDamageStep(gs, attackers, blockerMap, false)

	// Run SBAs.
	gameengine.StateBasedActions(gs)

	// Check invariants.
	violations := gameengine.RunAllInvariants(gs)
	if len(violations) > 0 {
		return &failure{
			CardName:    fmt.Sprintf("%s_vs_%s", kwA, kwB),
			Interaction: "keyword_matrix",
			Invariant:   violations[0].Name,
			Message:     violations[0].Message,
		}
	}

	return nil
}
