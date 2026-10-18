package gameengine

import (
	"fmt"
	"math/rand"
	"testing"
)

// r172 — the two board-wide observer scans fired per entering permanent.
// Two board shapes, because the stall board and the ordinary board are
// not the same question (r164: check the pathological case AND the
// normal one).

// r172Flags mirrors what a real battlefield permanent carries: an empty
// map makes mapaccess1_faststr a nil/count==0 early-return and would
// hide the exact cost being measured.
func r172Flags(extra ...string) map[string]int {
	f := map[string]int{
		"kw:haste":           1,
		"entered_this_turn":  1,
		"summoning_sickness": 0,
		"attacked_this_turn": 0,
	}
	for _, k := range extra {
		f[k] = 1
	}
	return f
}

// r172Board builds a seat-0 battlefield of n permanents. swarm=true makes
// them all creature tokens (the EP4/EP5 shape); swarm=false mixes in the
// lands and noncreature artifacts an ordinary board carries.
func r172Board(n int, swarm bool) (*GameState, *Permanent) {
	gs := NewGameState(4, rand.New(rand.NewSource(172)), nil)
	for i := 0; i < n; i++ {
		var p *Permanent
		if swarm || i%5 != 0 {
			p = addBattlefield(gs, 0, fmt.Sprintf("Goblin-%d", i), 1, 1, "creature", "goblin", "token")
		} else if i%10 == 0 {
			p = addBattlefield(gs, 0, fmt.Sprintf("Mountain-%d", i), 0, 0, "land")
		} else {
			p = addBattlefield(gs, 0, fmt.Sprintf("Relic-%d", i), 0, 0, "artifact")
		}
		p.Flags = r172Flags()
	}
	entering := addBattlefield(gs, 0, "Goblin-entering", 1, 1, "creature", "goblin", "token")
	entering.Flags = r172Flags()
	return gs, entering
}

func benchObserverScans(b *testing.B, n int, swarm bool) {
	gs, entering := r172Board(n, swarm)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FireEvolveTriggers(gs, 0, entering)
		FireSoulbondTriggers(gs, 0, entering)
	}
}

func BenchmarkR172_ObserverScans_Swarm120(b *testing.B) { benchObserverScans(b, 120, true) }
func BenchmarkR172_ObserverScans_Swarm400(b *testing.B) { benchObserverScans(b, 400, true) }
func BenchmarkR172_ObserverScans_Mixed60(b *testing.B)  { benchObserverScans(b, 60, false) }
func BenchmarkR172_ObserverScans_Mixed120(b *testing.B) { benchObserverScans(b, 120, false) }

// IsCreature on its own, over both shapes — it is called from far more
// than these two scans, so its ordering is a separate question.
func benchIsCreature(b *testing.B, n int, swarm bool) {
	gs, _ := r172Board(n, swarm)
	bf := gs.Seats[0].Battlefield
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range bf {
			if p.IsCreature() {
				continue
			}
		}
	}
}

func BenchmarkR172_IsCreature_Swarm120(b *testing.B) { benchIsCreature(b, 120, true) }
func BenchmarkR172_IsCreature_Mixed120(b *testing.B) { benchIsCreature(b, 120, false) }
