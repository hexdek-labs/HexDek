package main

// Module 2: Card Combo Pairs (--combo-pairs)
//
// Takes commonly-played cEDH/EDH staples from the oracle corpus.
// For each pair: places both on the battlefield, runs one full turn
// cycle (untap → upkeep → draw → main1 → combat → main2 → end),
// checks invariants after each phase.

import (
	"fmt"
	"runtime/debug"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// cEDH/EDH staple cards to test in pairs.
var stapleNames = []string{
	"Sol Ring", "Mana Crypt", "Rhystic Study", "Mystic Remora",
	"Thassa's Oracle", "Demonic Consultation", "Doomsday", "Food Chain",
	"Aetherflux Reservoir", "Isochron Scepter", "Dramatic Reversal",
	"Walking Ballista", "Necrotic Ooze", "Hermit Druid",
	"Sensei's Divining Top", "Cyclonic Rift", "Swords to Plowshares",
	"Path to Exile", "Counterspell", "Swan Song", "Pact of Negation",
	"Force of Will", "Mana Drain", "Toxic Deluge", "Damnation",
	"Wrath of God", "Doubling Season", "Parallel Lives",
	"Blood Artist", "Zulaport Cutthroat", "Grave Pact",
	"Dictate of Erebos", "Ashnod's Altar", "Phyrexian Altar",
	"Viscera Seer", "Skullclamp", "Lightning Greaves", "Swiftfoot Boots",
	"Birthing Pod", "Survival of the Fittest", "Sylvan Library",
	"Mystic Forge", "Bolas's Citadel", "Necropotence", "Ad Nauseam",
	"Eternal Witness", "Snapcaster Mage", "Fierce Guardianship",
	"Deflecting Swat", "Opposition Agent", "Drannith Magistrate",
	"Null Rod", "Collector Ouphe", "Cursed Totem", "Grand Abolisher",
	"Humility", "Rest in Peace", "Grafdigger's Cage", "Torpor Orb",
	"Hushbringer", "Aven Mindcensor",
}

func runComboPairs(corpus *astload.Corpus, oracleCards []*oracleCard) []failure {
	// Build lookup from oracle cards by name.
	oracleByName := map[string]*oracleCard{}
	for _, oc := range oracleCards {
		oracleByName[oc.Name] = oc
	}

	// Resolve staples against the corpus.
	type resolvedCard struct {
		oc  *oracleCard
		ast interface{} // we just need the oracle card data
	}
	var staples []*oracleCard
	for _, name := range stapleNames {
		if oc, ok := oracleByName[name]; ok {
			staples = append(staples, oc)
		}
	}

	if len(staples) < 2 {
		return nil // not enough cards found in corpus
	}

	var fails []failure

	// Test each unique pair (i < j to avoid duplicates).
	for i := 0; i < len(staples); i++ {
		for j := i + 1; j < len(staples); j++ {
			pairFails := testComboPair(corpus, staples[i], staples[j])
			fails = append(fails, pairFails...)
		}
	}

	return fails
}

func testComboPair(corpus *astload.Corpus, cardA, cardB *oracleCard) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("%s + %s", cardA.Name, cardB.Name),
				Interaction: "combo_pair",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v\n%s", r, string(stack)),
			})
		}
	}()

	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Step:   "",
		Flags:  map[string]int{},
		Cards:  corpus,
	}

	// 4 seats with basic setup.
	for i := 0; i < 4; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Idx:   i,
			Flags: map[string]int{},
		}
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler %d-%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		for j := 0; j < 3; j++ {
			seat.Hand = append(seat.Hand, &gameengine.Card{
				Name: fmt.Sprintf("HandCard %d-%d", i, j), Owner: i,
				Types: []string{"creature"},
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}

	// Place card A on seat 0's battlefield.
	astA, _ := corpus.Get(cardA.Name)
	typesA := cardA.Types
	if len(typesA) == 0 {
		typesA = []string{"creature"}
	}
	powA, toughA := cardA.Power, cardA.Toughness
	if toughA <= 0 {
		toughA = 1
		if powA <= 0 {
			powA = 1
		}
	}
	permA := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: cardA.Name, Owner: 0,
			Types: typesA, Colors: cardA.Colors,
			CMC: cardA.CMC, BasePower: powA, BaseToughness: toughA,
			AST: astA,
		},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, permA)

	// Place card B on seat 0's battlefield too.
	astB, _ := corpus.Get(cardB.Name)
	typesB := cardB.Types
	if len(typesB) == 0 {
		typesB = []string{"creature"}
	}
	powB, toughB := cardB.Power, cardB.Toughness
	if toughB <= 0 {
		toughB = 1
		if powB <= 0 {
			powB = 1
		}
	}
	permB := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: cardB.Name, Owner: 0,
			Types: typesB, Colors: cardB.Colors,
			CMC: cardB.CMC, BasePower: powB, BaseToughness: toughB,
			AST: astB,
		},
		Controller: 0, Owner: 0,
		Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, permB)

	// Also place a vanilla creature on seat 1 for interactions.
	opPerm := &gameengine.Permanent{
		Card: &gameengine.Card{
			Name: "Opponent Bear", Owner: 1,
			Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
		},
		Controller: 1, Owner: 1,
		Flags: map[string]int{},
	}
	gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, opPerm)

	gs.Snapshot()

	// Run through phases like testPhases does.
	pairName := fmt.Sprintf("%s + %s", cardA.Name, cardB.Name)

	phases := []struct {
		name string
		fn   func(gs *gameengine.GameState)
	}{
		{"untap", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "untap"
			gameengine.UntapAll(gs, gs.Active)
		}},
		{"upkeep", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "upkeep"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"draw", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "beginning", "draw"
			if len(gs.Seats[gs.Active].Library) > 0 {
				gs.Seats[gs.Active].Hand = append(gs.Seats[gs.Active].Hand,
					gs.Seats[gs.Active].Library[0])
				gs.Seats[gs.Active].Library = gs.Seats[gs.Active].Library[1:]
			}
			gameengine.StateBasedActions(gs)
		}},
		{"main1", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "precombat_main", ""
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"combat", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "combat", "beginning_of_combat"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"main2", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "postcombat_main", ""
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
		{"end_step", func(gs *gameengine.GameState) {
			gs.Phase, gs.Step = "ending", "end"
			gameengine.ScanExpiredDurations(gs, gs.Phase, gs.Step)
			gs.InvalidateCharacteristicsCache()
			gameengine.StateBasedActions(gs)
		}},
	}

	for _, p := range phases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fails = append(fails, failure{
						CardName:    pairName,
						Interaction: "combo_phase_" + p.name,
						Panicked:    true,
						PanicMsg:    fmt.Sprintf("%v", r),
					})
				}
			}()

			p.fn(gs)

			violations := gameengine.RunAllInvariants(gs)
			for _, v := range violations {
				fails = append(fails, failure{
					CardName:    pairName,
					Interaction: "combo_phase_" + p.name,
					Invariant:   v.Name,
					Message:     v.Message,
				})
			}
		}()
	}

	return fails
}
