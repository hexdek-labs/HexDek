package gameengine

import (
	"math/rand"
	"testing"
)

// r75 — standing benchmark for the cost the r70/r72 keyword migration
// added to the hot combat path, kept because throughput is now part of
// the gate for every migration batch.
//
// The shape matters. An earlier benchmark invalidated the cache on every
// single block check and reported ~10x, which is a worst case that does
// not occur: DeclareBlockers invalidates once and then checks every
// attacker x blocker pair, so each permanent misses the characteristics
// cache ONCE and hits thereafter. The alloc count confirms it — 120
// allocs for a 12x12 board is 24 permanents x 5, not 144 checks x N.
//
// Measured delta on that realistic shape: ~63.6us and ~9KB per
// DeclareBlockers. Against a ~1.07s game that is roughly 0.3% of game
// time, which is far too small to explain the grinder throughput
// variation filed as open — a negative result worth keeping, since it
// was the leading hypothesis and it is wrong.

func r75Board(n int) (*GameState, []*Permanent, []*Permanent) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	var atk, blk []*Permanent
	for i := 0; i < n; i++ {
		c := &Card{Name: "A", Owner: 0, BasePower: 2, BaseToughness: 2,
			Types: []string{"creature"}}
		p := &Permanent{Card: c, Controller: 0, Owner: 0}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		atk = append(atk, p)
		c2 := &Card{Name: "B", Owner: 1, BasePower: 2, BaseToughness: 2,
			Types: []string{"creature"}}
		q := &Permanent{Card: c2, Controller: 1, Owner: 1}
		gs.Seats[1].Battlefield = append(gs.Seats[1].Battlefield, q)
		blk = append(blk, q)
	}
	return gs, atk, blk
}

// REALISTIC: one invalidation, then every attacker x blocker pair checked,
// which is what DeclareBlockers actually does.
func BenchmarkR75_DeclareBlockersShape_LayerAware(b *testing.B) {
	gs, atk, blk := r75Board(12)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.InvalidateCharacteristicsCache()
		for _, a := range atk {
			for _, d := range blk {
				CanBlockCombatKeywords(gs, a, d)
			}
		}
	}
}

func BenchmarkR75_DeclareBlockersShape_LayerBlind(b *testing.B) {
	_, atk, blk := r75Board(12)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, a := range atk {
			for _, d := range blk {
				_ = a.HasKeyword("intimidate") || a.HasKeyword("fear") ||
					a.HasKeyword("shadow") || d.HasKeyword("shadow") ||
					a.HasKeyword("skulk") || a.HasKeyword("daunt")
			}
		}
	}
}
