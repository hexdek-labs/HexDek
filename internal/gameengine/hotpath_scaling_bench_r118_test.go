package gameengine

import (
	"fmt"
	"math/rand"
	"testing"
)

// r118 — board-size scaling sweep for the remaining per-turn hot paths.
//
// r112 swept the SBAs and r116 swept the untap step; both were O(n^2) in
// board size and both were found by the same shape (a battlefield walk
// nested inside a battlefield walk). The static detector
// `scripts/detect_nested_battlefield_walks_r116.go.txt` now reports only
// known-benign hits, so anything still superlinear is in code the detector
// cannot see — a map rebuilt per call, a slice grown quadratically, a sort
// per element. This sweep measures instead of inferring.
//
// NORMALISE BY THE INHERENT WORK. Trigger dispatch and the untap/cleanup
// scans are legitimately O(board), so they are reported per permanent;
// blocking and combat damage are legitimately O(attacker x blocker pairs),
// so they are reported per pair. A flat per-unit number is a linear
// algorithm with a constant factor. Only a CLIMBING per-unit number is
// superlinear (r113).

// r118Board spreads n untapped, non-sick 2/2 creatures across all four
// seats, which is the production shape (r116). Seat 0 is the active seat.
func r118Board(n int) *GameState {
	gs := NewGameState(4, rand.New(rand.NewSource(118)), nil)
	for i := 0; i < n; i++ {
		seat := i % 4
		c := &Card{Name: fmt.Sprintf("r118 Body %d", i), Owner: seat,
			BasePower: 2, BaseToughness: 2, Types: []string{"creature"}}
		p := &Permanent{Card: c, Controller: seat, Owner: seat,
			Counters: map[string]int{}, Flags: map[string]int{}}
		p.SummoningSick = false
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	}
	return gs
}

var r118Sizes = []int{40, 80, 160, 320, 640, 1280, 2560}

// ---------------------------------------------------------------- triggers

func BenchmarkR118_FirePhaseTriggers(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r118Board(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				FirePhaseTriggers(gs, "upkeep", "upkeep")
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/perm")
		})
	}
}

func BenchmarkR118_FireCardTrigger(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r118Board(n)
			ctx := map[string]interface{}{"gs": gs}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				FireCardTrigger(gs, "attack", ctx)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/perm")
		})
	}
}

func BenchmarkR118_ScanExpiredDurations(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r118Board(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ScanExpiredDurations(gs, "ending", "cleanup")
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/perm")
		})
	}
}

func BenchmarkR118_FireDelayedTriggers(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r118Board(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				FireDelayedTriggers(gs, "ending", "end")
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/perm")
		})
	}
}

// ------------------------------------------------------------------ combat

func BenchmarkR118_DeclareAttackers(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r118Board(n)
			gs.Phase = "combat"
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				DeclareAttackers(gs, 0)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(n), "ns/perm")
		})
	}
}

// Blocking and combat damage are O(attackers x blockers). Seat 0 attacks
// seat 1, so the pair count is (n/4)^2 and the per-pair figure is what has
// to stay flat.
func r118Combat(n int) (*GameState, []*Permanent, int) {
	gs := r118Board(n)
	gs.Phase = "combat"
	gs.Active = 0
	var attackers []*Permanent
	for _, p := range gs.Seats[0].Battlefield {
		setAttackerDefender(p, 1)
		p.Flags["attacking"] = 1
		attackers = append(attackers, p)
	}
	pairs := len(attackers) * len(gs.Seats[1].Battlefield)
	if pairs == 0 {
		pairs = 1
	}
	return gs, attackers, pairs
}

func BenchmarkR118_DeclareBlockers(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, attackers, pairs := r118Combat(n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				DeclareBlockers(gs, attackers, 1)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(pairs), "ns/pair")
		})
	}
}

func BenchmarkR118_DealCombatDamageStep(b *testing.B) {
	for _, n := range r118Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, attackers, pairs := r118Combat(n)
			blockers := DeclareBlockers(gs, attackers, 1)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				DealCombatDamageStep(gs, attackers, blockers, false)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(pairs), "ns/pair")
		})
	}
}
