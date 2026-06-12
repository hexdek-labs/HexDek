package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// r62 seed-determinism regression: the Yggdrasil noise RNG used to be
// seeded from the global math/rand at construction
// (rand.New(rand.NewSource(rand.Int63()))), so every targeting /
// selectAmongTop decision differed across replays of the same game
// seed — breaking Loki repro, ELO parity, and anticheat verification.
// The first ObserveEvent of a seeded game now reseeds the stream from
// (gs.Seed, seatIdx).

func noiseStream(t *testing.T, gameSeed int64, seatIdx int, n int) []float64 {
	t.Helper()
	h := NewYggdrasilHatWithNoise(nil, 0, 0.2)
	gs := gameengine.NewGameStateSeeded(2, gameSeed, nil)
	// Any event delivered before the first decision triggers the reseed —
	// in real games setup/draw/mulligan events precede all noise consumers.
	h.ObserveEvent(gs, seatIdx, &gameengine.Event{Kind: "draw"})
	out := make([]float64, n)
	for i := range out {
		out[i] = h.applyNoise(0)
	}
	return out
}

func TestYggdrasilNoise_DeterministicUnderGameSeed(t *testing.T) {
	a := noiseStream(t, 42, 0, 64)
	b := noiseStream(t, 42, 0, 64)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("noise stream diverged at draw %d for identical (seed, seat): %v vs %v", i, a[i], b[i])
		}
	}
}

func TestYggdrasilNoise_SeatAndSeedDecorrelated(t *testing.T) {
	base := noiseStream(t, 42, 0, 64)

	otherSeat := noiseStream(t, 42, 1, 64)
	if streamsEqual(base, otherSeat) {
		t.Fatal("seat 0 and seat 1 share an identical noise stream for the same game seed")
	}

	otherSeed := noiseStream(t, 43, 0, 64)
	if streamsEqual(base, otherSeed) {
		t.Fatal("different game seeds produced identical noise streams")
	}
}

// TestYggdrasilNoise_UnseededGameKeepsLegacyStream pins the conservative
// fallback: a game whose state carries Seed==0 must NOT reseed the
// constructor-initialized stream (legacy nondeterministic behavior for
// casual unseeded games is preserved on purpose).
func TestYggdrasilNoise_UnseededGameKeepsLegacyStream(t *testing.T) {
	h := NewYggdrasilHatWithNoise(nil, 0, 0.2)
	gs := gameengine.NewGameState(2, nil, nil) // Seed stays 0
	h.ObserveEvent(gs, 0, &gameengine.Event{Kind: "draw"})
	if h.noiseSeededFor != 0 {
		t.Fatalf("noiseSeededFor = %d for an unseeded game; want 0 (no deterministic reseed)", h.noiseSeededFor)
	}
}

func streamsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
