package gameengine

import (
	"math/rand"
	"strings"
	"testing"
)

// TestGenerateChaosDeck_SeedDeterministic pins the r63 seed-determinism
// fix in GenerateChaosDeck. The basic-land distribution used to range a
// map (the commander's color-identity set) to build the `colors` slice it
// round-robins over, so Go's randomized map-iteration order placed basic
// lands at different positions on every call — making the SAME seed
// produce a DIFFERENT deck, which breaks loki --seed reproduction and the
// correctness seed=42 sweeps. This test calls GenerateChaosDeck many
// times with a freshly-reseeded-but-identical RNG and asserts every
// resulting deck is byte-identical. Hermetic (no corpus/data files), so
// it runs in CI. Before the fix it fails with overwhelming probability
// (a multi-color ciSet randomizes per call); after, it always passes.
func TestGenerateChaosDeck_SeedDeterministic(t *testing.T) {
	// A two-color (B/G) commander so the colors slice has >1 element and
	// its order is observable in the basic-land layout.
	cc := &ChaosCorpus{
		LegendaryCreatures: []*ChaosCard{
			{Name: "Test Commander", Types: []string{"legendary", "creature"},
				Colors: []string{"B", "G"}, ColorIdentity: []string{"B", "G"},
				Power: 3, Toughness: 3, CMC: 4},
		},
		BasicLands: map[string]string{
			"W": "Plains", "U": "Island", "B": "Swamp", "R": "Mountain", "G": "Forest",
		},
	}
	// A handful of eligible nonbasic lands + nonland cards so the deck
	// fills out the way a real corpus would.
	for i := 0; i < 30; i++ {
		cc.NonBasicLands = append(cc.NonBasicLands, &ChaosCard{
			Name: "NBLand" + string(rune('A'+i)), Types: []string{"land"}, ColorIdentity: []string{"B"},
		})
	}
	for i := 0; i < 120; i++ {
		cc.NonLand = append(cc.NonLand, &ChaosCard{
			Name:  "Spell" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Types: []string{"creature"}, Colors: []string{"G"}, ColorIdentity: []string{"G"},
			Power: 2, Toughness: 2, CMC: 2,
		})
	}

	const seed = 42
	first := deckSig(GenerateChaosDeck(cc, rand.New(rand.NewSource(seed))))
	if first == "" {
		t.Fatal("GenerateChaosDeck returned nil/empty for the synthetic corpus")
	}
	for i := 0; i < 64; i++ {
		got := deckSig(GenerateChaosDeck(cc, rand.New(rand.NewSource(seed))))
		if got != first {
			t.Fatalf("GenerateChaosDeck is NONDETERMINISTIC across calls with the same seed (iteration %d):\n first=%s\n   got=%s",
				i, first, got)
		}
	}
}

func deckSig(d *ChaosDeck) string {
	if d == nil {
		return ""
	}
	return d.Commander.Name + "|" + strings.Join(d.Cards, ",")
}
