package gameengine

import (
	"fmt"
	"math/rand"
	"testing"
)

// r116 — board-size scaling for the UNTAP STEP.
//
// Found by a static detector for nested battlefield walks (the enforcement
// the r112 SBA SCAN CONTRACT never had). `UntapAll` calls
// `auraHoldsDownUntap` once per permanent, UNGATED, and that function walks
// every seat's entire battlefield looking for an attached aura carrying the
// `aura_no_untap` scaffold kind. r112's shape exactly, on a path that runs
// every turn of every game.
//
// The board is spread across all four seats, because that is the production
// shape: UntapAll walks one seat (~n/4) while the inner scan walks all of
// them (~n). Concentrating everything on seat 0 would overstate the cost.

func r116Board(n int) *GameState {
	gs := NewGameState(4, rand.New(rand.NewSource(116)), nil)
	for i := 0; i < n; i++ {
		seat := i % 4
		c := &Card{Name: fmt.Sprintf("r116 Body %d", i), Owner: seat,
			BasePower: 2, BaseToughness: 2, Types: []string{"creature"}}
		p := &Permanent{Card: c, Controller: seat, Owner: seat,
			Tapped: true, Counters: map[string]int{}, Flags: map[string]int{}}
		gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, p)
	}
	return gs
}

var r116Sizes = []int{40, 80, 160, 320, 640, 1280, 2560}

func BenchmarkR116_UntapAllByBoardSize(b *testing.B) {
	for _, n := range r116Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r116Board(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				UntapAll(gs, 0)
				// Re-tap so every iteration does the same work.
				for _, p := range gs.Seats[0].Battlefield {
					p.Tapped = true
				}
			}
		})
	}
}

// The replacement index on its own, to attribute the cost rather than infer
// it. Pre-fix this slot measured `auraHoldsDownUntap(gs, oneSubject)`, which
// was cleanly LINEAR per call (122.9ns at n=40 -> 12346ns at n=2560) — and
// UntapAll called it once per permanent, which is where the square came
// from. The index below does that one linear pass ONCE per untap step.
func BenchmarkR116_CollectAuraNoUntapHostsByBoardSize(b *testing.B) {
	for _, n := range r116Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r116Board(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = collectAuraNoUntapHosts(gs)
			}
		})
	}
}
