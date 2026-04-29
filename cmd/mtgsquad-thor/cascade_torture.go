package main

// cascade_torture.go — Trigger cascade stress tests.
//
// Sets up board states where a single action triggers a chain of
// effects: ETB triggers, death triggers, LTB triggers, damage triggers,
// state-based actions cascading into more triggers. Verifies the engine
// doesn't infinite loop, corrupt state, or miss triggers in deep chains.

import (
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runCascadeTorture(_ *astload.Corpus, _ []*oracleCard) []failure {
	start := time.Now()
	var fails []failure

	scenarios := []struct {
		name string
		test func() []failure
	}{
		{"etb_chain_10", cascadeETBChain},
		{"death_trigger_cascade", cascadeDeathTriggers},
		{"sba_loop_convergence", cascadeSBAConvergence},
		{"sacrifice_chain", cascadeSacrificeChain},
		{"bounce_retrigger", cascadeBounceRetrigger},
		{"flicker_chain", cascadeFlickerChain},
		{"mass_etb_50_tokens", cascadeMassETB},
		{"destroy_with_death_triggers", cascadeDestroyWithDeathTriggers},
	}

	for _, s := range scenarios {
		result := s.test()
		if len(result) > 0 {
			fails = append(fails, result...)
		}
		log.Printf("  cascade: %s — %d fails", s.name, len(result))
	}

	log.Printf("  cascade complete: %d scenarios, %d fails, %s",
		len(scenarios), len(fails), time.Since(start))
	return fails
}

func cascadeBaseState() *gameengine.GameState {
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
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Lib_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		gs.Seats = append(gs.Seats, seat)
	}
	return gs
}

func cascadeETBChain() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_etb_chain", Interaction: "cascade_etb",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Place 10 creatures one at a time, running ETB hooks + SBA each time.
	for i := 0; i < 10; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("ETBCreature_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		gameengine.StateBasedActions(gs)
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_etb_chain", Interaction: "cascade_etb",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeDeathTriggers() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_death", Interaction: "cascade_death_triggers",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Place 20 creatures, then kill them all via lethal damage.
	for i := 0; i < 20; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("Doomed_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}
	gs.Snapshot()
	// Mark all with lethal damage.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			p.MarkedDamage = p.Toughness() + 1
		}
	}
	gameengine.StateBasedActions(gs)
	// All should be dead.
	alive := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			alive++
		}
	}
	if alive > 0 {
		fails = append(fails, failure{
			CardName: "cascade_death", Interaction: "cascade_death_triggers",
			Message: fmt.Sprintf("%d creatures survived lethal damage", alive),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_death", Interaction: "cascade_death_triggers",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeSBAConvergence() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_sba_converge", Interaction: "cascade_sba",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Create a state that needs multiple SBA passes: creatures with 0
	// toughness, +1/+1 and -1/-1 counters to annihilate, etc.
	for i := 0; i < 10; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("ZeroTough_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 0,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{"counter:+1/+1": 3, "counter:-1/-1": 3},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_sba_converge", Interaction: "cascade_sba",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeSacrificeChain() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_sacrifice", Interaction: "cascade_sac_chain",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Place 15 creatures and sacrifice them one by one.
	var perms []*gameengine.Permanent
	for i := 0; i < 15; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("SacTarget_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		perms = append(perms, perm)
	}
	gs.Snapshot()
	for _, p := range perms {
		gameengine.SacrificePermanent(gs, p, "cascade_test")
		gameengine.StateBasedActions(gs)
	}
	if len(gs.Seats[0].Battlefield) != 0 {
		fails = append(fails, failure{
			CardName: "cascade_sacrifice", Interaction: "cascade_sac_chain",
			Message: fmt.Sprintf("battlefield should be empty, has %d", len(gs.Seats[0].Battlefield)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_sacrifice", Interaction: "cascade_sac_chain",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeBounceRetrigger() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_bounce", Interaction: "cascade_bounce_retrigger",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Place a creature, bounce it, replay it 5 times.
	card := &gameengine.Card{
		Name: "BounceMe", Owner: 0,
		Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
	}
	for cycle := 0; cycle < 5; cycle++ {
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		gameengine.StateBasedActions(gs)
		// Bounce it.
		gameengine.BouncePermanent(gs, perm, nil, "hand")
		gameengine.StateBasedActions(gs)
		// Remove from hand to simulate re-casting (card moves hand→stack→battlefield).
		hand := gs.Seats[0].Hand
		for i, c := range hand {
			if c == card {
				gs.Seats[0].Hand = append(hand[:i], hand[i+1:]...)
				break
			}
		}
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_bounce", Interaction: "cascade_bounce_retrigger",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeFlickerChain() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_flicker", Interaction: "cascade_flicker_chain",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	card := &gameengine.Card{
		Name: "FlickerTarget", Owner: 0,
		Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
	}
	perm := &gameengine.Permanent{
		Card: card, Controller: 0, Owner: 0,
		Timestamp: gs.NextTimestamp(),
		Counters:  map[string]int{}, Flags: map[string]int{},
	}
	gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	gs.Snapshot()
	// Flicker 10 times: exile + return.
	for i := 0; i < 10; i++ {
		gameengine.ExilePermanent(gs, perm, nil)
		gameengine.StateBasedActions(gs)
		// Remove from exile.
		exile := gs.Seats[0].Exile
		for j, c := range exile {
			if c == card {
				gs.Seats[0].Exile = append(exile[:j], exile[j+1:]...)
				break
			}
		}
		perm = &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		gameengine.InvokeETBHook(gs, perm)
		gameengine.StateBasedActions(gs)
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_flicker", Interaction: "cascade_flicker_chain",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeMassETB() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_mass_etb", Interaction: "cascade_mass_etb_50",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Slam 50 tokens onto the battlefield simultaneously, fire ETB for each.
	var perms []*gameengine.Permanent
	for i := 0; i < 50; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("MassToken_%d", i), Owner: 0,
			Types: []string{"creature", "token"}, BasePower: 1, BaseToughness: 1,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
		perms = append(perms, perm)
	}
	for _, p := range perms {
		gameengine.InvokeETBHook(gs, p)
	}
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_mass_etb", Interaction: "cascade_mass_etb_50",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}

func cascadeDestroyWithDeathTriggers() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "cascade_destroy_death", Interaction: "cascade_destroy_death",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := cascadeBaseState()
	// Place 20 creatures, destroy them all, fire phase triggers after.
	for i := 0; i < 20; i++ {
		card := &gameengine.Card{
			Name: fmt.Sprintf("Victim_%d", i), Owner: 0,
			Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
		}
		perm := &gameengine.Permanent{
			Card: card, Controller: 0, Owner: 0,
			Timestamp: gs.NextTimestamp(),
			Counters:  map[string]int{}, Flags: map[string]int{},
		}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, perm)
	}
	gs.Snapshot()
	toDestroy := make([]*gameengine.Permanent, len(gs.Seats[0].Battlefield))
	copy(toDestroy, gs.Seats[0].Battlefield)
	for _, p := range toDestroy {
		if p != nil {
			gameengine.DestroyPermanent(gs, p, nil)
		}
	}
	gameengine.StateBasedActions(gs)
	gameengine.FirePhaseTriggers(gs, "ending", "end")
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "cascade_destroy_death", Interaction: "cascade_destroy_death",
			Invariant: v.Name, Message: v.Message,
		})
	}
	return fails
}
