package main

// density_stress.go — Board density stress tests.
//
// Creates game states with extreme numbers of permanents (50, 100, 200+
// per seat) and runs interactions + invariants to verify the engine
// doesn't degrade, corrupt state, or miss SBA under load. Tests the
// "100 token" scenarios common in cEDH (Dockside, Krenko, Scute Swarm).

import (
	"fmt"
	"log"
	"time"

	"github.com/hexdek/hexdek/internal/astload"
	"github.com/hexdek/hexdek/internal/gameengine"
)

func runDensityStress(_ *astload.Corpus, oracleCards []*oracleCard) []failure {
	start := time.Now()
	var fails []failure

	scenarios := []struct {
		name  string
		count int
		test  func(count int) []failure
	}{
		{"50_creatures_sba", 50, densitySBA},
		{"100_tokens_destroy_all", 100, densityBoardWipe},
		{"200_permanents_invariants", 200, densityInvariants},
		{"50_creatures_fight_cascade", 50, densityFightCascade},
		{"100_mixed_exile_all", 100, densityExileAll},
		{"150_creatures_damage_sweep", 150, densityDamageSweep},
		{"50_creatures_counter_flood", 50, densityCounterFlood},
		{"100_creatures_steal_cascade", 100, densityStealCascade},
	}

	for _, s := range scenarios {
		result := s.test(s.count)
		if len(result) > 0 {
			fails = append(fails, result...)
		}
		log.Printf("  density: %s — %d fails", s.name, len(result))
	}

	log.Printf("  density complete: %d scenarios, %d fails, %s",
		len(scenarios), len(fails), time.Since(start))
	return fails
}

func makeDenseBoard(creaturesPerSeat int) *gameengine.GameState {
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
				Name: fmt.Sprintf("LibCard_%d_%d", i, j), Owner: i,
				Types: []string{"creature"}, BasePower: 1, BaseToughness: 1,
			})
		}
		for j := 0; j < creaturesPerSeat; j++ {
			card := &gameengine.Card{
				Name:          fmt.Sprintf("Token_%d_%d", i, j),
				Owner:         i,
				Types:         []string{"creature", "token"},
				BasePower:     2,
				BaseToughness: 2,
			}
			perm := &gameengine.Permanent{
				Card:       card,
				Controller: i,
				Owner:      i,
				Timestamp:  gs.NextTimestamp(),
				Counters:   map[string]int{},
				Flags:      map[string]int{},
			}
			seat.Battlefield = append(seat.Battlefield, perm)
		}
		gs.Seats = append(gs.Seats, seat)
	}
	gs.Snapshot()
	return gs
}

func densitySBA(count int) []failure {
	gs := makeDenseBoard(count)
	// Mark half the creatures with lethal damage.
	for i := 0; i < count/2; i++ {
		p := gs.Seats[0].Battlefield[i]
		p.MarkedDamage = p.Toughness() + 1
	}
	gameengine.StateBasedActions(gs)
	// Should have killed exactly count/2 creatures on seat 0.
	alive := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.IsCreature() {
			alive++
		}
	}
	expected := count - count/2
	var fails []failure
	if alive != expected {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_%d", count),
			Interaction: "density_sba",
			Message:     fmt.Sprintf("expected %d alive, got %d after lethal damage SBA", expected, alive),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_%d", count),
			Interaction: "density_sba",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityBoardWipe(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_%d", count),
				Interaction: "density_board_wipe",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Destroy all creatures on seat 0.
	toDestroy := make([]*gameengine.Permanent, len(gs.Seats[0].Battlefield))
	copy(toDestroy, gs.Seats[0].Battlefield)
	for _, p := range toDestroy {
		if p != nil {
			gameengine.DestroyPermanent(gs, p, nil)
		}
	}
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_%d", count),
			Interaction: "density_board_wipe",
			Message:     fmt.Sprintf("seat 0 should be empty after wipe, has %d permanents", len(gs.Seats[0].Battlefield)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_%d", count),
			Interaction: "density_board_wipe",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityInvariants(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_%d", count),
				Interaction: "density_invariants",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Run all phase transitions with a dense board.
	phases := []struct{ phase, step string }{
		{"beginning", "untap"},
		{"beginning", "upkeep"},
		{"beginning", "draw"},
		{"precombat_main", "main"},
		{"combat", "begin_of_combat"},
		{"combat", "declare_attackers"},
		{"postcombat_main", "main"},
		{"ending", "end"},
		{"ending", "cleanup"},
	}
	for _, p := range phases {
		gs.Phase, gs.Step = p.phase, p.step
		if p.step == "untap" {
			gameengine.UntapAll(gs, 0)
		}
		gameengine.StateBasedActions(gs)
		violations := gameengine.RunAllInvariants(gs)
		for _, v := range violations {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_%d_%s_%s", count, p.phase, p.step),
				Interaction: "density_phase_invariant",
				Invariant:   v.Name,
				Message:     v.Message,
			})
		}
	}
	return fails
}

func densityFightCascade(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_fight_%d", count),
				Interaction: "density_fight_cascade",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Make seat 0 creatures fight seat 1 creatures 1:1.
	for i := 0; i < count && i < len(gs.Seats[1].Battlefield); i++ {
		a := gs.Seats[0].Battlefield[i]
		b := gs.Seats[1].Battlefield[i]
		if a != nil && b != nil && a.IsCreature() && b.IsCreature() {
			a.MarkedDamage += b.Power()
			b.MarkedDamage += a.Power()
		}
	}
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_fight_%d", count),
			Interaction: "density_fight_cascade",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityExileAll(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_exile_%d", count),
				Interaction: "density_exile_all",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	toExile := make([]*gameengine.Permanent, len(gs.Seats[0].Battlefield))
	copy(toExile, gs.Seats[0].Battlefield)
	for _, p := range toExile {
		if p != nil {
			gameengine.ExilePermanent(gs, p, nil)
		}
	}
	gameengine.StateBasedActions(gs)
	if len(gs.Seats[0].Battlefield) != 0 {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_exile_%d", count),
			Interaction: "density_exile_all",
			Message:     fmt.Sprintf("seat 0 should be empty after exile, has %d", len(gs.Seats[0].Battlefield)),
		})
	}
	// Tokens cease to exist when they leave the battlefield (CR §111.8),
	// so exile should be empty after exiling tokens.
	if len(gs.Seats[0].Exile) != 0 {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_exile_%d", count),
			Interaction: "density_exile_all",
			Message:     fmt.Sprintf("tokens should cease to exist, exile has %d cards", len(gs.Seats[0].Exile)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_exile_%d", count),
			Interaction: "density_exile_all",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityDamageSweep(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_sweep_%d", count),
				Interaction: "density_damage_sweep",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Deal 1 damage to all creatures (shouldn't kill 2/2s).
	for _, seat := range gs.Seats {
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				p.MarkedDamage += 1
			}
		}
	}
	gameengine.StateBasedActions(gs)
	// All should survive (2/2 with 1 damage).
	alive0 := 0
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil {
			alive0++
		}
	}
	if alive0 != count {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_sweep_%d", count),
			Interaction: "density_damage_sweep",
			Message:     fmt.Sprintf("1 damage to 2/2s: expected %d alive, got %d", count, alive0),
		})
	}
	// Now deal 2 more (total 3 on 2-toughness).
	for _, seat := range gs.Seats {
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				p.MarkedDamage += 2
			}
		}
	}
	gameengine.StateBasedActions(gs)
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_sweep_%d", count),
			Interaction: "density_damage_sweep",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityCounterFlood(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_counters_%d", count),
				Interaction: "density_counter_flood",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Put +1/+1 counters on every creature, then -1/-1 to cancel.
	for _, p := range gs.Seats[0].Battlefield {
		if p != nil && p.Counters != nil {
			p.Counters["+1/+1"] = 5
			p.Counters["-1/-1"] = 5
		}
	}
	gameengine.StateBasedActions(gs)
	// After SBA counter annihilation (§704.5q), both should be 0.
	for _, p := range gs.Seats[0].Battlefield {
		if p == nil || p.Counters == nil {
			continue
		}
		plus := p.Counters["+1/+1"]
		minus := p.Counters["-1/-1"]
		if plus != 0 || minus != 0 {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_counters_%d", count),
				Interaction: "density_counter_flood",
				Message:     fmt.Sprintf("%s has +1/+1=%d -1/-1=%d after annihilation", p.Card.Name, plus, minus),
			})
			break
		}
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_counters_%d", count),
			Interaction: "density_counter_flood",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}

func densityStealCascade(count int) (fails []failure) {
	defer func() {
		if r := recover(); r != nil {
			fails = append(fails, failure{
				CardName:    fmt.Sprintf("density_steal_%d", count),
				Interaction: "density_steal_cascade",
				Panicked:    true,
				PanicMsg:    fmt.Sprintf("%v", r),
			})
		}
	}()
	gs := makeDenseBoard(count)
	// Steal all of seat 1's creatures to seat 0.
	toSteal := make([]*gameengine.Permanent, len(gs.Seats[1].Battlefield))
	copy(toSteal, gs.Seats[1].Battlefield)
	for _, p := range toSteal {
		if p == nil {
			continue
		}
		// Remove from seat 1.
		bf := gs.Seats[1].Battlefield
		for i, q := range bf {
			if q == p {
				gs.Seats[1].Battlefield = append(bf[:i], bf[i+1:]...)
				break
			}
		}
		p.Controller = 0
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	}
	gameengine.StateBasedActions(gs)
	// Seat 0 should have 2*count, seat 1 should have 0.
	if len(gs.Seats[1].Battlefield) != 0 {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_steal_%d", count),
			Interaction: "density_steal_cascade",
			Message:     fmt.Sprintf("seat 1 should be empty after steal, has %d", len(gs.Seats[1].Battlefield)),
		})
	}
	violations := gameengine.RunAllInvariants(gs)
	for _, v := range violations {
		fails = append(fails, failure{
			CardName:    fmt.Sprintf("density_steal_%d", count),
			Interaction: "density_steal_cascade",
			Invariant:   v.Name,
			Message:     v.Message,
		})
	}
	return fails
}
