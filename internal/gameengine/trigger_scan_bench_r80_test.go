package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r80 — standing benchmark for the per-permanent trigger scan.
//
// r79 added a layer-aware guard to permHasTriggerEvent and put it FIRST,
// so every permanent scanned paid a full layer-pipeline query. These
// callers scan every permanent on every land drop / ETB / spell cast,
// and almost none carry the trigger. Measured on prod: throughput fell
// to ~45% of baseline with gc/game up 4x. Gating the expensive check
// behind the cheap printed-AST scan is 12x faster and allocation-free,
// with identical semantics.
//
// Kept because this is the shape that bites: a correctness guard that is
// cheap in isolation and ruinous inside a hot scan.
//
// Realistic shape: FireLandfallTriggers scans every permanent on every
// land drop. Most permanents have no landfall trigger at all.
func r80Board(n int) (*GameState, []*Permanent) {
	gs := NewGameState(4, rand.New(rand.NewSource(7)), nil)
	var all []*Permanent
	for i := 0; i < n; i++ {
		c := &Card{Name: "Body", Owner: 0, BasePower: 2, BaseToughness: 2,
			Types: []string{"creature"}}
		c.AST = &gameast.CardAST{Abilities: []gameast.Ability{
			&gameast.Triggered{Trigger: gameast.Trigger{Event: "dies"}},
		}}
		p := &Permanent{Card: c, Controller: 0, Owner: 0}
		gs.Seats[0].Battlefield = append(gs.Seats[0].Battlefield, p)
		all = append(all, p)
	}
	return gs, all
}

func BenchmarkR80_ScanAllPermanentsForLandfall(b *testing.B) {
	gs, all := r80Board(40)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gs.InvalidateCharacteristicsCache() // a land dropped; state changed
		for _, p := range all {
			_ = permHasTriggerEvent(gs, p, "landfall")
		}
	}
}
