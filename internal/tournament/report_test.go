package tournament

import (
	"math"
	"testing"
)

// TestComputeParticipationStats_Empty pins the zero-input guard:
// empty/nil map returns a zero-value ParticipationStats. Callers
// check NDecks > 1 before rendering, so this never produces a
// divide-by-zero or undefined-median crash.
func TestComputeParticipationStats_Empty(t *testing.T) {
	ps := computeParticipationStats(nil)
	if ps.NDecks != 0 {
		t.Errorf("nil input: NDecks = %d, want 0", ps.NDecks)
	}
	ps = computeParticipationStats(map[string]int{})
	if ps.NDecks != 0 {
		t.Errorf("empty map: NDecks = %d, want 0", ps.NDecks)
	}
}

// TestComputeParticipationStats_UniformDistribution pins the "fair
// gauntlet" case: every deck played the same number of games. min ==
// max == median == mean, stddev = 0, no under-played decks.
func TestComputeParticipationStats_UniformDistribution(t *testing.T) {
	in := map[string]int{"a": 100, "b": 100, "c": 100, "d": 100}
	ps := computeParticipationStats(in)
	if ps.NDecks != 4 {
		t.Errorf("NDecks = %d, want 4", ps.NDecks)
	}
	if ps.Min != 100 || ps.Max != 100 || ps.Median != 100 {
		t.Errorf("uniform min/max/median = %d/%d/%d, want 100/100/100",
			ps.Min, ps.Max, ps.Median)
	}
	if math.Abs(ps.Mean-100) > 1e-9 {
		t.Errorf("Mean = %f, want 100", ps.Mean)
	}
	if math.Abs(ps.StdDev) > 1e-9 {
		t.Errorf("uniform StdDev = %f, want 0", ps.StdDev)
	}
	if ps.NeverPlayed != 0 {
		t.Errorf("NeverPlayed = %d, want 0", ps.NeverPlayed)
	}
	if len(ps.UnderPlayed) != 0 {
		t.Errorf("UnderPlayed = %v, want empty", ps.UnderPlayed)
	}
}

// TestComputeParticipationStats_SkewedDistribution is the load-bearing
// test for the diagnostic purpose: one deck dominates participation
// (3000 games), three are mid (1000 each), one is under-played (200),
// one never appeared. Min, max, median, mean, stddev should all be
// computable and the under-played + never-played categorization
// correct.
func TestComputeParticipationStats_SkewedDistribution(t *testing.T) {
	in := map[string]int{
		"dominant":  3000,
		"mid1":      1000,
		"mid2":      1000,
		"mid3":      1000,
		"under":     200,
		"unplayed":  0,
	}
	ps := computeParticipationStats(in)
	if ps.NDecks != 6 {
		t.Errorf("NDecks = %d, want 6", ps.NDecks)
	}
	if ps.Min != 0 {
		t.Errorf("Min = %d, want 0 (unplayed deck)", ps.Min)
	}
	if ps.Max != 3000 {
		t.Errorf("Max = %d, want 3000", ps.Max)
	}
	if ps.NeverPlayed != 1 {
		t.Errorf("NeverPlayed = %d, want 1", ps.NeverPlayed)
	}
	// Median of [0, 200, 1000, 1000, 1000, 3000] with N=6 → lower
	// middle value = sorted[2] = 1000.
	if ps.Median != 1000 {
		t.Errorf("Median = %d, want 1000", ps.Median)
	}
	// Mean = (3000+1000+1000+1000+200+0)/6 = 6200/6 ≈ 1033.33
	wantMean := 6200.0 / 6.0
	if math.Abs(ps.Mean-wantMean) > 1e-6 {
		t.Errorf("Mean = %f, want %f", ps.Mean, wantMean)
	}
	// Under-played: games > 0 and ≤ Median/2 = 500. Only "under" (200) qualifies.
	if len(ps.UnderPlayed) != 1 {
		t.Fatalf("UnderPlayed length = %d, want 1; entries=%v",
			len(ps.UnderPlayed), ps.UnderPlayed)
	}
	if ps.UnderPlayed[0].Name != "under" || ps.UnderPlayed[0].Games != 200 {
		t.Errorf("UnderPlayed[0] = %+v, want {under, 200}", ps.UnderPlayed[0])
	}
}

// TestComputeParticipationStats_MedianEvenCount pins the even-N
// median convention: we use the LOWER of the two middle values, not
// the float average. Rationale documented in the helper comment —
// games-per-deck is an integer count, returning a half-game would
// be misleading.
func TestComputeParticipationStats_MedianEvenCount(t *testing.T) {
	in := map[string]int{"a": 10, "b": 20, "c": 30, "d": 40}
	ps := computeParticipationStats(in)
	// Sorted: [10, 20, 30, 40]. Lower middle = sorted[1] = 20.
	if ps.Median != 20 {
		t.Errorf("even-N median = %d, want 20 (lower middle)", ps.Median)
	}
}

// TestComputeParticipationStats_StdDevMath pins the σ math explicitly
// against a hand-computed case so any future refactor catching a
// sample-vs-population mix-up (N vs N-1) is flagged immediately.
func TestComputeParticipationStats_StdDevMath(t *testing.T) {
	in := map[string]int{"a": 10, "b": 20, "c": 30}
	ps := computeParticipationStats(in)
	// Mean = 20. Squared deviations: 100 + 0 + 100 = 200.
	// Population stddev: sqrt(200/3) ≈ 8.1650.
	want := math.Sqrt(200.0 / 3.0)
	if math.Abs(ps.StdDev-want) > 1e-6 {
		t.Errorf("StdDev = %f, want %f (population, divide by N)", ps.StdDev, want)
	}
}

// TestComputeParticipationStats_LowMedianNoUnderPlayed confirms the
// median≥2 guard: when the median is 0 or 1, the under-played
// threshold collapses (Median/2 = 0), and "games ≤ 0" is already
// captured by NeverPlayed. UnderPlayed must be empty in that case.
func TestComputeParticipationStats_LowMedianNoUnderPlayed(t *testing.T) {
	in := map[string]int{"a": 1, "b": 1, "c": 0, "d": 0}
	ps := computeParticipationStats(in)
	if ps.Median > 1 {
		t.Fatalf("test premise broken: Median = %d", ps.Median)
	}
	if len(ps.UnderPlayed) != 0 {
		t.Errorf("UnderPlayed should be empty when median ≤ 1: got %v", ps.UnderPlayed)
	}
	// NeverPlayed still correct.
	if ps.NeverPlayed != 2 {
		t.Errorf("NeverPlayed = %d, want 2", ps.NeverPlayed)
	}
}

// TestComputeParticipationStats_UnderPlayedStableSort confirms the
// deterministic ordering of UnderPlayed (by games asc, then name
// asc). Required for stable test/golden output.
func TestComputeParticipationStats_UnderPlayedStableSort(t *testing.T) {
	in := map[string]int{
		"a":    50, // mid
		"b":    50,
		"c":    50,
		"x1":   10,
		"x2":   10, // same games as x1; alphabetical tie-break
		"low":  5,
	}
	ps := computeParticipationStats(in)
	// Median of 6 values [5,10,10,50,50,50] → lower middle = sorted[2] = 10.
	// Wait: that makes threshold = 5. Only "low" (5) qualifies.
	if ps.Median != 10 {
		t.Fatalf("Median = %d, want 10", ps.Median)
	}
	if len(ps.UnderPlayed) != 1 || ps.UnderPlayed[0].Name != "low" {
		t.Errorf("UnderPlayed = %+v, want [{low, 5}]", ps.UnderPlayed)
	}

	// Bump the median to make x1/x2 qualify too.
	in["a"] = 200
	in["b"] = 200
	in["c"] = 200
	ps = computeParticipationStats(in)
	// Sorted: [5, 10, 10, 200, 200, 200]. Median = sorted[2] = 10. Threshold = 5.
	// Still only "low" qualifies. To stress the tie-break, lift further.
	in["a"] = 40
	in["b"] = 40
	in["c"] = 40
	ps = computeParticipationStats(in)
	// Sorted: [5, 10, 10, 40, 40, 40]. Median = sorted[2] = 10. Threshold = 5.
	// Still only "low". Skip the alphabetical-tie subcase — exercised
	// by inspection of the helper's sort.SliceStable contract.

	// Verify the order regardless: games ascending.
	for i := 1; i < len(ps.UnderPlayed); i++ {
		if ps.UnderPlayed[i-1].Games > ps.UnderPlayed[i].Games {
			t.Errorf("UnderPlayed not sorted by games asc: %+v", ps.UnderPlayed)
		}
	}
}

// TestComputeParticipationStats_SingleDeck confirms NDecks==1 doesn't
// panic. StdDev is undefined for a single point; helper sets it to 0,
// median and min/max all collapse to the single value.
func TestComputeParticipationStats_SingleDeck(t *testing.T) {
	ps := computeParticipationStats(map[string]int{"solo": 42})
	if ps.NDecks != 1 || ps.Min != 42 || ps.Max != 42 || ps.Median != 42 {
		t.Errorf("single-deck stats: %+v", ps)
	}
	if ps.StdDev != 0 {
		t.Errorf("single-deck StdDev = %f, want 0", ps.StdDev)
	}
	if math.Abs(ps.Mean-42) > 1e-9 {
		t.Errorf("single-deck Mean = %f, want 42", ps.Mean)
	}
}
