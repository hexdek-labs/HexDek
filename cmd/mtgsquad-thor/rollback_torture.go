package main

// rollback_torture.go — State rollback integrity tests.
//
// Serializes game state mid-turn, restores it, and verifies invariants
// pass on the restored state. Tests deep copy integrity, pointer
// aliasing, and state consistency after save/restore cycles.

import (
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runRollbackTorture(_ *astload.Corpus, _ []*oracleCard) []failure {
	start := time.Now()
	var fails []failure

	scenarios := []struct {
		name string
		test func() []failure
	}{
		{"snapshot_restore_basic", rollbackBasic},
		{"snapshot_after_combat", rollbackAfterCombat},
		{"snapshot_after_destroy", rollbackAfterDestroy},
		{"snapshot_after_exile", rollbackAfterExile},
		{"snapshot_deep_copy_isolation", rollbackDeepCopyIsolation},
		{"snapshot_counter_state", rollbackCounterState},
		{"snapshot_graveyard_state", rollbackGraveyardState},
		{"snapshot_multiple_cycles", rollbackMultipleCycles},
	}

	for _, s := range scenarios {
		result := s.test()
		if len(result) > 0 {
			fails = append(fails, result...)
		}
		log.Printf("  rollback: %s — %d fails", s.name, len(result))
	}

	log.Printf("  rollback complete: %d scenarios, %d fails, %s",
		len(scenarios), len(fails), time.Since(start))
	return fails
}

func rollbackBaseState() *gameengine.GameState {
	gs := &gameengine.GameState{
		Turn: 1, Active: 0, Phase: "precombat_main", Step: "",
		Flags: map[string]int{},
	}
	for i := 0; i < 2; i++ {
		seat := &gameengine.Seat{Life: 40, Flags: map[string]int{}}
		for j := 0; j < 10; j++ {
			seat.Library = append(seat.Library, &gameengine.Card{
				Name: fmt.Sprintf("Lib_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 2, BaseToughness: 2,
			})
		}
		for j := 0; j < 5; j++ {
			card := &gameengine.Card{
				Name: fmt.Sprintf("BF_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 3, BaseToughness: 3,
			}
			perm := &gameengine.Permanent{
				Card: card, Controller: i, Owner: i,
				Timestamp: gs.NextTimestamp(),
				Counters:  map[string]int{}, Flags: map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, perm)
		}
		gs.Seats = append(gs.Seats, seat)
	}
	gs.Snapshot()
	return gs
}

func cloneGameState(gs *gameengine.GameState) *gameengine.GameState {
	clone := &gameengine.GameState{
		Turn:   gs.Turn,
		Active: gs.Active,
		Phase:  gs.Phase,
		Step:   gs.Step,
		Flags:  make(map[string]int),
	}
	for k, v := range gs.Flags {
		clone.Flags[k] = v
	}
	for _, s := range gs.Seats {
		if s == nil {
			clone.Seats = append(clone.Seats, nil)
			continue
		}
		ns := &gameengine.Seat{
			Life:  s.Life,
			Lost:  s.Lost,
			Flags: make(map[string]int),
			Idx:   s.Idx,
		}
		for k, v := range s.Flags {
			ns.Flags[k] = v
		}
		for _, c := range s.Library {
			ns.Library = append(ns.Library, c.DeepCopy())
		}
		for _, c := range s.Hand {
			ns.Hand = append(ns.Hand, c.DeepCopy())
		}
		for _, c := range s.Graveyard {
			ns.Graveyard = append(ns.Graveyard, c.DeepCopy())
		}
		for _, c := range s.Exile {
			ns.Exile = append(ns.Exile, c.DeepCopy())
		}
		for _, p := range s.Battlefield {
			if p == nil {
				continue
			}
			np := &gameengine.Permanent{
				Card:          p.Card.DeepCopy(),
				Controller:    p.Controller,
				Owner:         p.Owner,
				Tapped:        p.Tapped,
				SummoningSick: p.SummoningSick,
				PhasedOut:     p.PhasedOut,
				MarkedDamage:  p.MarkedDamage,
				Timestamp:     p.Timestamp,
				Counters:      make(map[string]int),
				Flags:         make(map[string]int),
			}
			for k, v := range p.Counters {
				np.Counters[k] = v
			}
			for k, v := range p.Flags {
				np.Flags[k] = v
			}
			ns.Battlefield = append(ns.Battlefield, np)
		}
		clone.Seats = append(clone.Seats, ns)
	}
	return clone
}

func rollbackBasic() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_basic", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	clone := cloneGameState(gs)
	// Verify clone passes invariants.
	violations := gameengine.RunAllInvariants(clone)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "rollback_basic", Interaction: "rollback",
			Invariant: v.Name, Message: "clone: " + v.Message,
		})
	}
	// Modify original, verify clone is unaffected.
	gs.Seats[0].Life = 0
	if clone.Seats[0].Life != 40 {
		fails = append(fails, failure{
			CardName: "rollback_basic", Interaction: "rollback",
			Message: "clone life changed when original was modified",
		})
	}
	return fails
}

func rollbackAfterCombat() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_combat", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	// Run combat.
	gs.Phase, gs.Step = "combat", "declare_attackers"
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			p.Tapped = true
			if p.Flags == nil {
				p.Flags = map[string]int{}
			}
			p.Flags["attacking"] = 1
		}
	}
	gameengine.StateBasedActions(gs)
	// Clone after combat.
	clone := cloneGameState(gs)
	violations := gameengine.RunAllInvariants(clone)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "rollback_combat", Interaction: "rollback",
			Invariant: v.Name, Message: "post-combat clone: " + v.Message,
		})
	}
	return fails
}

func rollbackAfterDestroy() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_destroy", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	// Destroy a creature.
	if len(gs.Seats[0].Battlefield) > 0 {
		gameengine.DestroyPermanent(gs, gs.Seats[0].Battlefield[0], nil)
		gameengine.StateBasedActions(gs)
	}
	clone := cloneGameState(gs)
	delete(clone.Flags, "_zone_conservation_total")
	violations := gameengine.RunAllInvariants(clone)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "rollback_destroy", Interaction: "rollback",
			Invariant: v.Name, Message: "post-destroy clone: " + v.Message,
		})
	}
	return fails
}

func rollbackAfterExile() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_exile", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	if len(gs.Seats[1].Battlefield) > 0 {
		gameengine.ExilePermanent(gs, gs.Seats[1].Battlefield[0], nil)
		gameengine.StateBasedActions(gs)
	}
	clone := cloneGameState(gs)
	delete(clone.Flags, "_zone_conservation_total")
	violations := gameengine.RunAllInvariants(clone)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName: "rollback_exile", Interaction: "rollback",
			Invariant: v.Name, Message: "post-exile clone: " + v.Message,
		})
	}
	return fails
}

func rollbackDeepCopyIsolation() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_isolation", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	clone := cloneGameState(gs)
	// Modify original's cards — clone should be unaffected.
	if len(gs.Seats[0].Battlefield) > 0 {
		gs.Seats[0].Battlefield[0].Card.Name = "MODIFIED"
		gs.Seats[0].Battlefield[0].Card.Types = []string{"artifact"}
		gs.Seats[0].Battlefield[0].MarkedDamage = 999
	}
	// Verify clone's card is unchanged.
	if len(clone.Seats[0].Battlefield) > 0 {
		p := clone.Seats[0].Battlefield[0]
		if p.Card.Name == "MODIFIED" {
			fails = append(fails, failure{
				CardName: "rollback_isolation", Interaction: "rollback",
				Message: "clone's card name was modified by original mutation — pointer aliasing",
			})
		}
		if p.MarkedDamage == 999 {
			fails = append(fails, failure{
				CardName: "rollback_isolation", Interaction: "rollback",
				Message: "clone's damage was modified by original mutation",
			})
		}
	}
	return fails
}

func rollbackCounterState() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_counters", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	// Add counters.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil {
			p.Counters["+1/+1"] = 3
		}
	}
	clone := cloneGameState(gs)
	// Remove counters from original.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil {
			delete(p.Counters, "+1/+1")
		}
	}
	// Clone should still have counters.
	for _, p := range clone.Seats[0].Battlefield {
		if p != nil && p.Counters["+1/+1"] != 3 {
			fails = append(fails, failure{
				CardName: "rollback_counters", Interaction: "rollback",
				Message: "clone's counters were modified by original mutation",
			})
			break
		}
	}
	return fails
}

func rollbackGraveyardState() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_graveyard", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	// Mill some cards to graveyard.
	for i := 0; i < 5 && len(gs.Seats[0].Library) > 0; i++ {
		c := gs.Seats[0].Library[0]
		gs.Seats[0].Library = gs.Seats[0].Library[1:]
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, c)
	}
	clone := cloneGameState(gs)
	// Clear original's graveyard.
	gs.Seats[0].Graveyard = nil
	// Clone should still have 5 cards.
	if len(clone.Seats[0].Graveyard) != 5 {
		fails = append(fails, failure{
			CardName: "rollback_graveyard", Interaction: "rollback",
			Message: fmt.Sprintf("clone graveyard should have 5, has %d", len(clone.Seats[0].Graveyard)),
		})
	}
	return fails
}

func rollbackMultipleCycles() (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName: "rollback_cycles", Interaction: "rollback",
				Panicked: true, PanicMsg: fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := rollbackBaseState()
	// Clone → modify → clone → modify → verify each clone is independent.
	var clones []*gameengine.GameState
	for cycle := 0; cycle < 5; cycle++ {
		clone := cloneGameState(gs)
		clones = append(clones, clone)
		// Modify original.
		gs.Seats[0].Life -= 5
		if len(gs.Seats[0].Battlefield) > 0 {
			gs.Seats[0].Battlefield[0].MarkedDamage += 1
		}
	}
	// Each clone should have the life total from when it was taken.
	for i, c := range clones {
		expectedLife := 40 - (i * 5)
		if c.Seats[0].Life != expectedLife {
			fails = append(fails, failure{
				CardName: "rollback_cycles", Interaction: "rollback",
				Message: fmt.Sprintf("clone %d: expected life %d, got %d", i, expectedLife, c.Seats[0].Life),
			})
		}
	}
	return fails
}
