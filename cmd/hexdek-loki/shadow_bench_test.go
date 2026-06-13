package main

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
	"github.com/hexdek/hexdek/internal/judge"
)

// buildBenchState constructs a representative mid-game GameState: 4 seats,
// each with `perSeat` minted creatures on the battlefield plus a few cards
// in graveyard/exile, so RunAllInvariants does realistic per-zone work
// (the InstanceID census, attachment scan, combat-legality scan, etc.).
// When `violate` is set it plants a duplicate Card pointer across two
// graveyards so RunAllInvariants emits violations through the Judge router
// every call — exercising the SHADOW sink's per-violation increment, the
// only path that touches the shadow mutex.
func buildBenchState(perSeat int, violate bool) *gameengine.GameState {
	gs := gameengine.NewGameState(4, nil, nil)
	for si := 0; si < 4; si++ {
		seat := gs.Seats[si]
		seat.Life = 40
		for i := 0; i < perSeat; i++ {
			c := &gameengine.Card{
				Name:          "Bench Creature",
				Owner:         si,
				Types:         []string{"creature"},
				BasePower:     2,
				BaseToughness: 2,
			}
			gameengine.MintOGInstanceID(gs, c)
			seat.Battlefield = append(seat.Battlefield, &gameengine.Permanent{
				Card:       c,
				Controller: si,
				Owner:      si,
				Timestamp:  gs.NextTimestamp(),
			})
		}
		for i := 0; i < 5; i++ {
			g := &gameengine.Card{Name: "Bench GY", Owner: si, Types: []string{"sorcery"}}
			gameengine.MintOGInstanceID(gs, g)
			seat.Graveyard = append(seat.Graveyard, g)
		}
	}
	if violate {
		dup := &gameengine.Card{Name: "Bench Dup", Owner: 0}
		gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard, dup)
		gs.Seats[1].Graveyard = append(gs.Seats[1].Graveyard, dup)
	}
	gs.Turn = 10
	return gs
}

// BenchmarkInvariants_Baseline measures parallel throughput of bare
// RunAllInvariants on a VIOLATING board with NO shadow sink registered —
// the pre-SHADOW path. Each goroutine owns its own GameState, exactly as
// loki's workers do (the characteristics cache is per-gs and not
// goroutine-safe to share).
func BenchmarkInvariants_Baseline(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		gs := buildBenchState(25, true)
		for pb.Next() {
			_ = gameengine.RunAllInvariants(gs)
		}
	})
}

// BenchmarkInvariants_Shadow measures the SAME parallel workload with the
// shadow sink registered, so every emitted violation drives the sink's
// per-name increment (mutex held only for the map write). The delta from
// the baseline is the entire steady-state cost of the SHADOW change on a
// violation-heavy board; on CLEAN games RunAllInvariants never calls
// LogViolation, so the sink is never invoked and the cost is exactly zero.
func BenchmarkInvariants_Shadow(b *testing.B) {
	unregister := judge.RegisterSink(shadowSink)
	defer unregister()
	b.RunParallel(func(pb *testing.PB) {
		gs := buildBenchState(25, true)
		for pb.Next() {
			_ = gameengine.RunAllInvariants(gs)
		}
	})
}
