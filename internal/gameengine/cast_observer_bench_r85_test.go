package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r85 — standing benchmark for the cast-observer scan.
//
// FireCastTriggerObservers walks every battlefield permanent on every
// spell cast, and calls ApplyIncrementTriggers first, which walks the
// caster's battlefield again. r85 made three reads inside those loops
// layer-aware (prowess, extort, increment, plus the type gates in front
// of two of them). That is the r80 shape by construction, so it is
// measured rather than argued about.
//
// Measured on an M4, 40 permanents, before -> after r85:
//
//	cold   31µs ->  73µs   (7 -> 247 allocs)
//	warm   20µs ->  20µs   (7 ->   7 allocs)
//
// The whole cost is the first layer query after an invalidation: it
// recomputes the board's characteristics, and every query behind it in
// the same cast is a map read. So the question the benchmark cannot
// answer — and the prod throughput gate can — is how often the cache is
// actually cold at cast time in a real game.
//
// If a gate ever shows a drop here, the lever is NOT to re-order these
// short-circuits (there is no cheap pre-filter that is a true superset
// of the layer answer: a keyword can only be ADDED by a continuous
// effect, so no raw read can disqualify a permanent). It is to make
// GetEffectiveCharacteristics cheap when no continuous effect applies.
func r85CastBoard(n int) (*GameState, *Card) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	for i := 0; i < n; i++ {
		c := &Card{Name: "Body", Owner: 0, BasePower: 2, BaseToughness: 2,
			Types: []string{"creature"}}
		c.AST = &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{Trigger: gameast.Trigger{Event: "dies"}},
		}}
		p := &Permanent{Card: c, Controller: 0, Owner: 0,
			Counters: map[string]int{}, Flags: map[string]int{}}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
	}
	spell := &Card{Name: "Bolt", Owner: 0, Types: []string{"instant"}, Colors: []string{"R"}}
	return gs, spell
}

// Pessimistic: something invalidated the cache since the last cast.
func BenchmarkR85_FireCastObserversCold(b *testing.B) {
	gs, spell := r85CastBoard(40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.InvalidateCharacteristicsCache()
		FireCastTriggerObservers(gs, spell, 0, false)
	}
}

// Optimistic: nothing changed between casts.
func BenchmarkR85_FireCastObserversWarm(b *testing.B) {
	gs, spell := r85CastBoard(40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FireCastTriggerObservers(gs, spell, 0, false)
	}
}
