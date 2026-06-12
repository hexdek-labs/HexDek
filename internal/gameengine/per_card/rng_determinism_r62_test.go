package per_card

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// r62 seed-determinism regression tests.
//
// Background: vial_smasher, zndrsplt, viconia, niv_mizzet_reborn and
// atraxa_grand drew from the process-global math/rand source. With
// Loki's parallel workers the global stream interleaves across
// concurrent games, so any game containing one of those cards was not
// reproducible from its seed (`--games N --seed S`) and its outcome
// varied with --workers. The handlers now route through rngIntn /
// rngShuffle, which prefer gs.Rng.

// TestRngHelpers_DeterministicUnderGameSeed pins that the helpers are a
// pure function of the game seed.
func TestRngHelpers_DeterministicUnderGameSeed(t *testing.T) {
	draw := func(seed int64) []int {
		gs := gameWithSeededRNG(t, 2, seed)
		out := make([]int, 64)
		for i := range out {
			out[i] = rngIntn(gs, 1000)
		}
		return out
	}

	a := draw(42)
	b := draw(42)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("rngIntn diverged at draw %d for identical seed: %d vs %d", i, a[i], b[i])
		}
	}

	c := draw(43)
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("rngIntn produced an identical 64-draw stream for different seeds")
	}

	shuffle := func(seed int64) []int {
		gs := gameWithSeededRNG(t, 2, seed)
		perm := make([]int, 32)
		for i := range perm {
			perm[i] = i
		}
		rngShuffle(gs, len(perm), func(i, j int) { perm[i], perm[j] = perm[j], perm[i] })
		return perm
	}
	p1 := shuffle(42)
	p2 := shuffle(42)
	for i := range p1 {
		if p1[i] != p2[i] {
			t.Fatalf("rngShuffle diverged at index %d for identical seed", i)
		}
	}
}

// TestViconia_RandomReturnDeterministicUnderSeed pins that Viconia's
// "return a creature card at random" pick is reproducible from the game
// seed — the handler used to draw from the global source.
func TestViconia_RandomReturnDeterministicUnderSeed(t *testing.T) {
	run := func(seed int64) string {
		gs := gameWithSeededRNG(t, 2, seed)
		perm := addPerm(gs, 0, "Viconia, Drow Apostate", "creature")
		names := []string{"Bear A", "Bear B", "Bear C", "Bear D", "Bear E"}
		for _, n := range names {
			gs.Seats[0].Graveyard = append(gs.Seats[0].Graveyard,
				newCardWithTypes(n, 0, "creature"))
		}
		viconiaDrowUpkeep(gs, perm, map[string]interface{}{"active_seat": 0})
		if len(gs.Seats[0].Hand) != 1 {
			t.Fatalf("expected exactly 1 card returned to hand, got %d", len(gs.Seats[0].Hand))
		}
		return gs.Seats[0].Hand[0].DisplayName()
	}

	first := run(1234)
	second := run(1234)
	if first != second {
		t.Fatalf("Viconia pick nondeterministic under identical seed: %q vs %q", first, second)
	}
}

// TestPerCardNoBareGlobalRand is a source gate: handlers must not draw
// from the process-global math/rand source. New randomness goes through
// rngIntn / rngShuffle (rng.go) so it stays on the per-game stream.
//
// Allowlisted files contain a documented gs.Rng-preferring fallback to
// the global source for callers that never seeded gs.Rng; anything else
// matching is a regression.
func TestPerCardNoBareGlobalRand(t *testing.T) {
	allow := map[string]bool{
		"rng.go": true, // the helper's own documented fallback
	}
	// Global-source draw functions. rand.New / rand.NewSource are fine —
	// they construct local generators.
	re := regexp.MustCompile(`\brand\.(Intn|Int31n?|Int63n?|Int\b|Uint32|Uint64|Float32|Float64|Perm|Shuffle|Seed|NormFloat64|ExpFloat64)\(`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") || allow[name] {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if re.MatchString(line) {
				t.Errorf("%s:%d draws from global math/rand — use rngIntn/rngShuffle (gs.Rng) instead: %s",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
