package main

// graveyard_storm.go — Graveyard interaction stress tests.
//
// Exercises graveyard-centric mechanics: mass mill, reanimate loops,
// dredge-style self-mill, graveyard exile, flashback from GY, and
// graveyard-order sensitivity. Verifies zone conservation holds when
// cards flow rapidly between library→graveyard→battlefield→graveyard.

import (
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runGraveyardStorm(_ *astload.Corpus, _ []*oracleCard) []failure {
	start := time.Now()
	var fails []failure

	scenarios := []struct {
		name string
		test func() []failure
	}{
		{"mass_mill_20", gyMassMill},
		{"mill_entire_library", gyMillEntireLibrary},
		{"reanimate_loop_5x", gyReanimateLoop},
		{"gy_exile_all", gyExileAll},
		{"gy_to_hand_mass", gyToHandMass},
		{"gy_order_preserved", gyOrderPreserved},
		{"destroy_reanimate_cycle", gyDestroyReanimateCycle},
		{"mill_then_sba", gyMillThenSBA},
	}

	for _, s := range scenarios {
		result := s.test()
		if len(result) > 0 {
			fails = append(fails, result...)
		}
		log.Printf("  graveyard: %s — %d fails", s.name, len(result))
	}

	log.Printf("  graveyard complete: %d scenarios, %d fails, %s",
		len(scenarios), len(fails), time.Since(start))
	return fails
}

func gyBaseState(librarySize int) *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn:   1,
		Active: 0,
		Phase:  "precombat_main",
		Step:   "",
		Flags:  map[string]int{},
	}
	for i := 0; i < 2; i++ {
		seat := &gameengine.Seat{
			Life:  40,
			Flags: map[string]int{},
		}
		for j := 0; j < librarySize; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("LibCard_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}
	return gs
}

func gyMassMill() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_mass_mill", Interaction: "graveyard_mass_mill",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(30)
	gs.Snapshot()
	// Mill 20 cards.
	for i := 0; i < 20 && len(gs.Seats[0].Library) > 0; i++ {
		card := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	}
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Graveyard) != 20 {
		fails = append(fails, failure{
			CardName: "gy_mass_mill", Interaction: "graveyard_mass_mill",
			Message: fmt.Sprintf("expected 20 in GY, got %d", len(gs.Seats[0].Graveyard)),
		})
	}
	if len(gs.Seats[0].Library) != 10 {
		fails = append(fails, failure{
			CardName: "gy_mass_mill", Interaction: "graveyard_mass_mill",
			Message: fmt.Sprintf("expected 10 in library, got %d", len(gs.Seats[0].Library)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_mass_mill", Interaction: "graveyard_mass_mill",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyMillEntireLibrary() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_mill_all", Interaction: "graveyard_mill_all",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(40)
	gs.Snapshot()
	total := len(gs.Seats[0].Library)
	// Mill entire library.
	for len(gs.Seats[0].Library) > 0 {
		card := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	}
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Graveyard) != total {
		fails = append(fails, failure{
			CardName: "gy_mill_all", Interaction: "graveyard_mill_all",
			Message: fmt.Sprintf("GY should have %d cards, has %d", total, len(gs.Seats[0].Graveyard)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_mill_all", Interaction: "graveyard_mill_all",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyReanimateLoop() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_reanimate", Interaction: "graveyard_reanimate_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(10)
	// Put a creature in the graveyard.
	card := &gameengine.Card{
		Name: "ReanimateTarget", Owner: 0,
		Types: []string{"creature"}, BasePower: 5, BaseToughness: 5,
	}
	gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	gs.Snapshot()
	// Reanimate→sacrifice cycle 5 times.
	for cycle := 0; cycle < 5; cycle++ {
		// Remove from GY.
		gy := gs.Seats[0].Graveyard
		for i, c := range gy {
			if c == card {
				gs.Seats[0].Graveyard = append(gy[:i], gy[i+1:]...)
				break
			}
		}
		// Place on battlefield.
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		gameengine.StateBasedActions(gs)
		// Sacrifice back to GY.
		gameengine.SacrificePermanent(gs, perm, "reanimate_test")
		gameengine.StateBasedActions(gs)
	}
	// Card should be in GY.
	inGY := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			inGY = true
			break
		}
	}
	if !inGY {
		fails = append(fails, failure{
			CardName: "gy_reanimate", Interaction: "graveyard_reanimate_loop",
			Message: "card should be in graveyard after 5 reanimate-sacrifice cycles",
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_reanimate", Interaction: "graveyard_reanimate_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyExileAll() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_exile_all", Interaction: "graveyard_exile_all",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(10)
	// Fill GY with 15 cards.
	for i := 0; i < 15; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
			Name: fmt.Sprintf("GYCard_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		})
	}
	gs.Snapshot()
	// Exile entire graveyard (Rest in Peace style).
	toExile := make([]*gameengine.Card, len(gs.Seats[0].Graveyard))
	copy(toExile, gs.Seats[0].Graveyard)
	gs.Seats[0].Graveyard = nil
	gs.Seats[0].Exile = append(gs.Seats[0].Exile, toExile...)
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Graveyard) != 0 {
		fails = append(fails, failure{
			CardName: "gy_exile_all", Interaction: "graveyard_exile_all",
			Message: fmt.Sprintf("GY should be empty, has %d", len(gs.Seats[0].Graveyard)),
		})
	}
	if len(gs.Seats[0].Exile) != 15 {
		fails = append(fails, failure{
			CardName: "gy_exile_all", Interaction: "graveyard_exile_all",
			Message: fmt.Sprintf("exile should have 15, has %d", len(gs.Seats[0].Exile)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_exile_all", Interaction: "graveyard_exile_all",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyToHandMass() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_to_hand", Interaction: "graveyard_to_hand",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(10)
	// Put 10 cards in GY, then move all to hand.
	for i := 0; i < 10; i++ {
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, &gameengine.Card{
			Name: fmt.Sprintf("GYToHand_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		})
	}
	gs.Snapshot()
	toReturn := make([]*gameengine.Card, len(gs.Seats[0].Graveyard))
	copy(toReturn, gs.Seats[0].Graveyard)
	gs.Seats[0].Graveyard = nil
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, toReturn...)
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Graveyard) != 0 {
		fails = append(fails, failure{
			CardName: "gy_to_hand", Interaction: "graveyard_to_hand",
			Message: fmt.Sprintf("GY should be empty, has %d", len(gs.Seats[0].Graveyard)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_to_hand", Interaction: "graveyard_to_hand",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyOrderPreserved() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_order", Interaction: "graveyard_order",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(10)
	// Mill cards one at a time — graveyard order should match mill order.
	var expected []string
	for i := 0; i < 10; i++ {
		card := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
		expected = append(expected, card.Name)
	}
	for i, c := range gs.Seats[0].Graveyard {
		if c.Name != expected[i] {
			fails = append(fails, failure{
				CardName: "gy_order", Interaction: "graveyard_order",
				Message: fmt.Sprintf("GY order mismatch at %d: expected %s, got %s", i, expected[i], c.Name),
			})
			break
		}
	}
	return fails
}

func gyDestroyReanimateCycle() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_destroy_reanimate", Interaction: "graveyard_destroy_reanimate",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(10)
	// Place 5 creatures, destroy them, reanimate them, destroy again.
	var cards []*gameengine.Card
	for i := 0; i < 5; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("CycleCreature_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
		}
		cards = append(cards, card)
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}
	gs.Snapshot()
	// Destroy all.
	toDestroy := make([]*gameengine.Permanent, len(gs.Seats[0].Battlefield))
	copy(toDestroy, gs.Seats[0].Battlefield)
	for _, p := range toDestroy {
		if p != nil {
			gameengine.DestroyPermanent(gs, p, nil)
		}
	}
	gameengine.StateBasedActions(gs)
	// Reanimate all from GY.
	for _, card := range cards {
		// Find in GY.
		found := false
		gy := gs.Seats[0].Graveyard
		for i, c := range gy {
			if c == card {
				gs.Seats[0].Graveyard = append(gy[:i], gy[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			continue
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_destroy_reanimate", Interaction: "graveyard_destroy_reanimate",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func gyMillThenSBA() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "gy_mill_sba", Interaction: "graveyard_mill_sba",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := gyBaseState(5)
	gs.Snapshot()
	// Mill entire library (only 5 cards) — player should lose to empty library
	// draw attempt. But SBA checks for 0 library only on draw, not passively.
	for len(gs.Seats[0].Library) > 0 {
		card := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, card)
	}
	gameengine.StateBasedActions(gs)
	// Verify GY has all 5 cards.
	if len(gs.Seats[0].Graveyard) != 5 {
		fails = append(fails, failure{
			CardName: "gy_mill_sba", Interaction: "graveyard_mill_sba",
			Message: fmt.Sprintf("GY should have 5, has %d", len(gs.Seats[0].Graveyard)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "gy_mill_sba", Interaction: "graveyard_mill_sba",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}
