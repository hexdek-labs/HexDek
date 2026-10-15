package gameengine

import (
	"fmt"
	"testing"
)

// r112 — board-size SCALING for the engine's hot per-permanent scans.
//
// r111 showed the pathological grinder games are not longer in TURNS
// (+24%) but ~24x more expensive PER TURN (median 11.16 s/turn against
// 0.47). That makes "what does a turn cost as the board grows?" the
// question, and nothing measured it: the standing benchmarks r80 and r85
// are both pinned at n=40.
//
// Their fixtures are already parameterized, so this sweeps them instead of
// building new ones. r85's own doc comment describes the shape to check:
//
//	FireCastTriggerObservers walks every battlefield permanent on every
//	spell cast, and calls ApplyIncrementTriggers first, which walks the
//	caster's battlefield again.
//
// A nested walk is quadratic unless something bounds the inner one. That
// is a claim in a comment; this measures it.
//
// Run:  go test ./internal/gameengine/ -run XXX -bench 'R112' -benchtime 50x

var r112Sizes = []int{10, 20, 40, 80, 160, 320}

func BenchmarkR112_TriggerScanByBoardSize(b *testing.B) {
	for _, n := range r112Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, all := r80Board(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				for _, p := range all {
					_ = permHasTriggerEvent(gs, p, "landfall")
				}
			}
		})
	}
}

func BenchmarkR112_CastObserversColdByBoardSize(b *testing.B) {
	for _, n := range r112Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, spell := r85CastBoard(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				FireCastTriggerObservers(gs, spell, 0, false)
			}
		})
	}
}

func BenchmarkR112_CastObserversWarmByBoardSize(b *testing.B) {
	for _, n := range r112Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, spell := r85CastBoard(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				FireCastTriggerObservers(gs, spell, 0, false)
			}
		})
	}
}

// The thing r85's comment names as "the whole cost": the first layer query
// after an invalidation, which recomputes the board's characteristics.
func BenchmarkR112_EffectiveCharsColdByBoardSize(b *testing.B) {
	for _, n := range r112Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, all := r80Board(n)
			subj := all[0]
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				_ = GetEffectiveCharacteristics(gs, subj)
			}
		})
	}
}

// StateBasedActions runs after every resolution — the other per-turn scan.
func BenchmarkR112_StateBasedActionsByBoardSize(b *testing.B) {
	for _, n := range r112Sizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, _ := r80Board(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				StateBasedActions(gs)
			}
		})
	}
}

// SBA is the one that came back superlinear in the first sweep, so it gets
// a longer arm to pin the exponent rather than reading it off five points.
var r112BigSizes = []int{40, 80, 160, 320, 640, 1280, 2560}

func BenchmarkR112_StateBasedActionsDeep(b *testing.B) {
	for _, n := range r112BigSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs, _ := r80Board(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				StateBasedActions(gs)
			}
		})
	}
}

// r80Board names every permanent "Body". If any SBA groups or compares by
// NAME across the battlefield (the legend rule is the obvious candidate),
// that fixture is pathological in a way a real board is not, and the
// superlinearity would be mine rather than the engine's. r100's lesson in
// reverse: a fixture that CREATES a coincidence the real path does not have
// is no better than one that omits a field.
//
// Same board, distinct names. If the curve flattens, the first result is a
// fixture artifact and must be withdrawn.
func r112UniqueNameBoard(n int) *GameState {
	gs, all := r80Board(n)
	for i, p := range all {
		p.Card.Name = fmt.Sprintf("r112 Body %d", i)
	}
	return gs
}

func BenchmarkR112_StateBasedActionsUniqueNames(b *testing.B) {
	for _, n := range r112BigSizes {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			gs := r112UniqueNameBoard(n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				gs.InvalidateCharacteristicsCache()
				StateBasedActions(gs)
			}
		})
	}
}
