package matchmaking

import (
	"math/rand"
	"testing"
)

// ---------------------------------------------------------------------------
// seedDensityScore primitive
// ---------------------------------------------------------------------------

// TestSeedDensityScore_FullWhenEnoughPeers pins the "no penalty"
// boundary: a bracket with exactly nSeats-1 same-bracket peers
// (i.e. enough to fill the pod) returns 1.0.
func TestSeedDensityScore_FullWhenEnoughPeers(t *testing.T) {
	pool := []DeckEntry{
		{Index: 0, Bracket: 3},
		{Index: 1, Bracket: 3},
		{Index: 2, Bracket: 3},
		{Index: 3, Bracket: 3}, // 4 total, candidate + 3 peers
		{Index: 4, Bracket: 5},
	}
	got := seedDensityScore(pool, 3, 4)
	if got != 1.0 {
		t.Fatalf("4 B3 decks (3 peers) for nSeats=4: want 1.0, got %v", got)
	}
}

// TestSeedDensityScore_SurplusPeersStillFull confirms the clamp:
// having MORE than nSeats-1 same-bracket peers doesn't push score
// above 1.0 (would over-weight popular brackets and bias seeding).
func TestSeedDensityScore_SurplusPeersStillFull(t *testing.T) {
	pool := make([]DeckEntry, 20)
	for i := range pool {
		pool[i] = DeckEntry{Index: i, Bracket: 3}
	}
	got := seedDensityScore(pool, 3, 4)
	if got != 1.0 {
		t.Fatalf("20 B3 decks for nSeats=4: want clamped 1.0, got %v", got)
	}
}

// TestSeedDensityScore_PartialPenalty pins the linear-penalty arm:
// 2 peers out of 3 needed → 0.667.
func TestSeedDensityScore_PartialPenalty(t *testing.T) {
	pool := []DeckEntry{
		{Index: 0, Bracket: 3}, // candidate
		{Index: 1, Bracket: 3},
		{Index: 2, Bracket: 3}, // 2 peers
		{Index: 3, Bracket: 5},
		{Index: 4, Bracket: 5},
		{Index: 5, Bracket: 5},
	}
	got := seedDensityScore(pool, 3, 4)
	want := 2.0 / 3.0
	if got < want-1e-9 || got > want+1e-9 {
		t.Fatalf("2 of 3 peers: want %v, got %v", want, got)
	}
}

// TestSeedDensityScore_SingletonZero pins the floor: a bracket with
// zero same-bracket peers gets score 0 — the seeding pick will not
// fire for this candidate so it cannot anchor a pod with no
// like-bracket fill.
func TestSeedDensityScore_SingletonZero(t *testing.T) {
	pool := []DeckEntry{
		{Index: 0, Bracket: 1}, // lone B1
		{Index: 1, Bracket: 5},
		{Index: 2, Bracket: 5},
		{Index: 3, Bracket: 5},
		{Index: 4, Bracket: 5},
	}
	got := seedDensityScore(pool, 1, 4)
	if got != 0.0 {
		t.Fatalf("singleton B1 in B5-heavy pool: want 0.0, got %v", got)
	}
}

// TestSeedDensityScore_BracketlessIsUnaffected pins the additive
// contract: decks without bracket metadata (bracket == 0) skip the
// density check entirely. Defends pools that mix unbracketed decks
// from being penalized for a feature they don't opt into.
func TestSeedDensityScore_BracketlessIsUnaffected(t *testing.T) {
	pool := []DeckEntry{
		{Index: 0, Bracket: 0}, // unbracketed candidate
		{Index: 1, Bracket: 5},
		{Index: 2, Bracket: 5},
		{Index: 3, Bracket: 5},
	}
	got := seedDensityScore(pool, 0, 4)
	if got != 1.0 {
		t.Fatalf("bracketless seed must return 1.0, got %v", got)
	}
}

// TestSeedDensityScore_DegenerateNSeats handles the trivial case
// where nSeats <= 1 — no peer demand, all candidates pass.
func TestSeedDensityScore_DegenerateNSeats(t *testing.T) {
	pool := []DeckEntry{{Index: 0, Bracket: 1}}
	if got := seedDensityScore(pool, 1, 1); got != 1.0 {
		t.Fatalf("nSeats=1: want 1.0, got %v", got)
	}
	if got := seedDensityScore(pool, 1, 0); got != 1.0 {
		t.Fatalf("nSeats=0: want 1.0, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// AssembleBracketPod end-to-end behavior
// ---------------------------------------------------------------------------

// TestAssembleBracketPod_SingletonOutlierDoesNotSeed is the load-
// bearing regression test for the original bug shape: one high-σ
// B1 deck dropped into an otherwise B5-heavy pool. Before the fix
// the B1's info gain plus the 0.1 cross-bracket floor could produce
// a pod where the B1 player gets 3 cEDH opponents. After the fix
// the B1 seed is depreferred (seedDensityScore = 0) so it does not
// anchor pods — over a statistically meaningful run of trials, the
// B1 should rarely seed and should rarely appear in the same pod
// as multiple B5s.
func TestAssembleBracketPod_SingletonOutlierDoesNotSeed(t *testing.T) {
	// Build a pool with 1 high-σ B1 outlier + 12 well-clustered B5
	// decks. The outlier's σ inflates its info gain above the B5s.
	pool := []DeckEntry{
		{Index: 0, Commander: "Casual Bear Tribal", Mu: 25, Sigma: 8.33, Games: 0, Bracket: 1},
	}
	for i := 1; i <= 12; i++ {
		pool = append(pool, DeckEntry{
			Index:     i,
			Commander: "cEDH " + string(rune('A'+i-1)),
			Mu:        30 + float64(i)*0.2,
			Sigma:     2.0,
			Games:     200,
			Bracket:   5,
		})
	}

	seededAsOutlier := 0
	outlierInPod := 0
	const trials = 200
	for trial := 0; trial < trials; trial++ {
		rng := rand.New(rand.NewSource(int64(trial)))
		got := AssembleBracketPod(rng, pool, 4)
		// Index 0 is the B1 outlier. The first slot returned is the
		// seed pick.
		if len(got) > 0 && got[0] == 0 {
			seededAsOutlier++
		}
		for _, idx := range got {
			if idx == 0 {
				outlierInPod++
				break
			}
		}
	}

	// With seedDensityScore=0 for the singleton B1, it should never
	// seed. Allow 0 strictly — any non-zero seed count indicates the
	// density gate leaked.
	if seededAsOutlier != 0 {
		t.Errorf("singleton B1 seeded %d/%d pods — density gate should drive this to 0", seededAsOutlier, trials)
	}
	// The outlier MAY still appear in pods (via the bracketWeight=0.1
	// non-seed pick). Tolerated, but should be rare — well below 50%.
	if outlierInPod > trials/2 {
		t.Errorf("singleton B1 appeared in %d/%d pods — bracket weighting should keep this rare", outlierInPod, trials)
	}
}

// TestAssembleBracketPod_DenseBracketSeedsMost confirms the inverse:
// a bracket with many same-bracket peers should produce most of the
// seeds across trials. Pins that the density signal is doing real
// work, not just a no-op.
func TestAssembleBracketPod_DenseBracketSeedsMost(t *testing.T) {
	// 1 lone B1 + 8 B3s + 2 B5s. B3 has the most peers (7 others)
	// AND clears the nSeats-1=3 threshold; B5 has only 1 peer so it
	// gets seedDensityScore = 1/3 ≈ 0.33.
	pool := []DeckEntry{
		{Index: 0, Mu: 25, Sigma: 6, Games: 5, Bracket: 1},
	}
	for i := 1; i <= 8; i++ {
		pool = append(pool, DeckEntry{Index: i, Mu: 28, Sigma: 4, Games: 30, Bracket: 3})
	}
	pool = append(pool,
		DeckEntry{Index: 9, Mu: 35, Sigma: 3, Games: 50, Bracket: 5},
		DeckEntry{Index: 10, Mu: 36, Sigma: 3, Games: 50, Bracket: 5},
	)

	seedBrackets := map[int]int{}
	const trials = 200
	for trial := 0; trial < trials; trial++ {
		rng := rand.New(rand.NewSource(int64(trial * 7)))
		got := AssembleBracketPod(rng, pool, 4)
		if len(got) > 0 {
			seedBrackets[pool[indexOfPoolEntry(pool, got[0])].Bracket]++
		}
	}

	if seedBrackets[3] < trials*70/100 {
		t.Errorf("B3 should seed ≥70%% of pods (dense bracket + threshold met), got %d/%d (%.1f%%)",
			seedBrackets[3], trials, 100*float64(seedBrackets[3])/float64(trials))
	}
	if seedBrackets[1] != 0 {
		t.Errorf("singleton B1 should never seed, got %d/%d", seedBrackets[1], trials)
	}
}

// TestAssembleBracketPod_AllUnbracketedUnchanged confirms the
// pre-existing behavior on pools without bracket metadata: every
// candidate gets seedDensityScore=1.0 so seed selection collapses to
// pure info-gain weighting — exactly what the test pool in
// matchmaking_test.go assumes.
func TestAssembleBracketPod_AllUnbracketedUnchanged(t *testing.T) {
	pool := newPool(10) // newPool sets Bracket=0 implicitly
	seeds := map[int]int{}
	const trials = 100
	for trial := 0; trial < trials; trial++ {
		rng := rand.New(rand.NewSource(int64(trial)))
		got := AssembleBracketPod(rng, pool, 4)
		if len(got) != 4 {
			t.Fatalf("trial %d: pod size %d, want 4", trial, len(got))
		}
		seeds[got[0]]++
	}
	// With all bracketless decks and identical mu/sigma, every deck
	// has equal seed probability. We don't pin uniformity strictly
	// (small-sample variance), but every deck should seed at least
	// once across 100 trials.
	if len(seeds) < 5 {
		t.Errorf("bracketless pool should distribute seeds broadly, only %d distinct seeds in %d trials",
			len(seeds), trials)
	}
}

// TestAssembleBracketPod_SmallPoolReturnsAll defends the existing
// short-circuit path: pool ≤ nSeats returns every deck regardless
// of bracket. The density signal must not interfere with this.
func TestAssembleBracketPod_SmallPoolReturnsAll(t *testing.T) {
	pool := []DeckEntry{
		{Index: 0, Bracket: 1},
		{Index: 1, Bracket: 5}, // mixed bracket but pool == nSeats
		{Index: 2, Bracket: 3},
		{Index: 3, Bracket: 5},
	}
	rng := rand.New(rand.NewSource(1))
	got := AssembleBracketPod(rng, pool, 4)
	if len(got) != 4 {
		t.Fatalf("want 4 entries, got %d", len(got))
	}
}

// indexOfPoolEntry returns the slice position of the pool entry whose
// Index matches the given value. Used by tests that need to look up
// the original DeckEntry from an AssembleBracketPod result.
func indexOfPoolEntry(pool []DeckEntry, idx int) int {
	for i, d := range pool {
		if d.Index == idx {
			return i
		}
	}
	return -1
}
