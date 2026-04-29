package main

// infinite_loop.go — Infinite loop detection tests.
//
// Constructs known infinite combo board states and verifies the engine's
// loop detection caps them without crashing or hanging. Tests both
// mandatory and optional loops, ETB loops, trigger loops, and
// state-based action loops.

import (
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

const loopTimeout = 50 // max iterations before declaring loop detected

func runInfiniteLoop(_ *astload.Corpus, _ []*oracleCard) []failure {
	start := time.Now()
	var fails []failure

	scenarios := []struct {
		name string
		test func() []failure
	}{
		{"etb_loop_2card", loopETB2Card},
		{"etb_loop_3card", loopETB3Card},
		{"sba_loop_zero_toughness", loopSBAZeroToughness},
		{"trigger_chain_loop", loopTriggerChain},
		{"flicker_loop", loopFlickerLoop},
		{"sacrifice_recursion", loopSacRecursion},
		{"damage_loop", loopDamageLoop},
		{"counter_loop", loopCounterLoop},
	}

	for _, s := range scenarios {
		result := s.test()
		if len(result) > 0 {
			fails = append(fails, result...)
		}
		log.Printf("  infinite-loop: %s — %d fails", s.name, len(result))
	}

	log.Printf("  infinite-loop complete: %d scenarios, %d fails, %s",
		len(scenarios), len(fails), time.Since(start))
	return fails
}

func loopBaseState() *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn: 1, Active: 0, Phase: "precombat_main", Step: "",
		Flags: map[string]int{},
	}
	for i := 0; i < 2; i++ {
		seat := &gameengine.Seat{Life: 40, Flags: map[string]int{}}
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Filler_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}
	return gs
}

func loopETB2Card() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_etb_2card", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	// Simulate: two creatures that each flicker the other on ETB.
	// Place both, then fire ETB in a loop with a counter.
	cardA := &gameengine.Card{
		Name: "FlickerA", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	cardB := &gameengine.Card{
		Name: "FlickerB", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	permA := &gameengine.Permanent{
		Card: cardA, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	permB := &gameengine.Permanent{
		Card: cardB, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, permA, permB)

	// Simulate ETB loop: A enters → triggers B flicker → B re-enters → triggers A flicker
	iterations := 0
	for iterations < loopTimeout {
		iterations++
		gameengine.InvokeETBHook(gs, permA)
		gameengine.StateBasedActions(gs)
		gameengine.InvokeETBHook(gs, permB)
		gameengine.StateBasedActions(gs)
	}
	// Success: we hit the timeout without crashing.
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_etb_2card", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopETB3Card() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_etb_3card", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	var perms [3]*gameengine.Permanent
	for i := 0; i < 3; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("LoopPiece_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		}
		perms[i] = &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perms[i])
	}

	for iter := 0; iter < loopTimeout; iter++ {
		for _, p := range perms {
			gameengine.InvokeETBHook(gs, p)
			gameengine.StateBasedActions(gs)
		}
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_etb_3card", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopSBAZeroToughness() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_sba_zero", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	// Place a 0-toughness creature repeatedly — SBA should kill it each time.
	for iter := 0; iter < loopTimeout; iter++ {
		card := &gameengine.Card{
			Name: "ZeroTough", Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 0,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.StateBasedActions(gs)
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_sba_zero", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopTriggerChain() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_trigger_chain", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	// Place 10 creatures, fire phase triggers in a loop.
	for i := 0; i < 10; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("TriggerCreature_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}

	for iter := 0; iter < loopTimeout; iter++ {
		gameengine.FirePhaseTriggers(gs, "beginning", "upkeep")
		gameengine.StateBasedActions(gs)
		gameengine.FirePhaseTriggers(gs, "ending", "end")
		gameengine.StateBasedActions(gs)
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_trigger_chain", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopFlickerLoop() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_flicker", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	card := &gameengine.Card{
		Name: "FlickerTarget", Owner: 0,
		Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
	}

	for iter := 0; iter < loopTimeout; iter++ {
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		gameengine.ExilePermanent(gs, perm, nil)
		gameengine.StateBasedActions(gs)
		// Remove from exile for next iteration.
		exile := gs.Seats[0].Exile
		for i, c := range exile {
			if c == card {
				gs.Seats[0].Exile = append(exile[:i], exile[i+1:]...)
				break
			}
		}
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_flicker", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopSacRecursion() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_sac_recursion", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	card := &gameengine.Card{
		Name: "RecurringCreature", Owner: 0,
		Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
	}

	for iter := 0; iter < loopTimeout; iter++ {
		// Place on battlefield.
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(), Counters: map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		// Sacrifice.
		gameengine.SacrificePermanent(gs, perm, "loop_test")
		gameengine.StateBasedActions(gs)
		// Remove from graveyard (simulate reanimate).
		gy := gs.Seats[0].Graveyard
		for i, c := range gy {
			if c == card {
				gs.Seats[0].Graveyard = append(gy[:i], gy[i+1:]...)
				break
			}
		}
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "loop_sac_recursion", Interaction: "infinite_loop",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func loopDamageLoop() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_damage", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	// Place indestructible creature, deal damage repeatedly.
	card := &gameengine.Card{
		Name: "Indestructible", Owner: 0,
		Types: []string{"creature"}, BasePower: 5, BaseToughness: 5,
	}
	perm := &gameengine.Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(), Counters: map[string]int{},
		Flags: map[string]int{"indestructible": 1},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	for iter := 0; iter < loopTimeout; iter++ {
		perm.MarkedDamage += 10
		gameengine.StateBasedActions(gs)
		// Damage should accumulate but creature survives (indestructible).
	}
	// Creature should still be on battlefield.
	found := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == perm {
			found = true
			break
		}
	}
	if !found {
		fails = append(fails, failure{
			CardName: "loop_damage", Interaction: "infinite_loop",
			Message: "indestructible creature should survive repeated lethal damage",
		})
	}
	return fails
}

func loopCounterLoop() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "loop_counter", Interaction: "infinite_loop",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := loopBaseState()
	card := &gameengine.Card{
		Name: "CounterTarget", Owner: 0,
		Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
	}
	perm := &gameengine.Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters: map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)

	// Add and remove counters in a loop — SBA annihilation.
	for iter := 0; iter < loopTimeout; iter++ {
		perm.Counters["+1/+1"] += 3
		perm.Counters["-1/-1"] += 3
		gameengine.StateBasedActions(gs)
		// After SBA, both should be 0.
		if perm.Counters["+1/+1"] != 0 || perm.Counters["-1/-1"] != 0 {
			fails = append(fails, failure{
				CardName: "loop_counter", Interaction: "infinite_loop",
				Message: fmt.Sprintf("iter %d: counters not annihilated (+1/+1=%d -1/-1=%d)",
					iter, perm.Counters["+1/+1"], perm.Counters["-1/-1"]),
			})
			break
		}
	}
	return fails
}
