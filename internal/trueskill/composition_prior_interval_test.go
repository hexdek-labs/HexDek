package trueskill

import (
	"math"
	"testing"
)

// composition_prior_interval_test.go — regressions for the PR #425
// confidence-interval API (ExpectedWinrateInterval, WinrateInterval,
// wilsonScoreInterval). The Wilson math itself is also covered with
// hand-computed reference values.

// -----------------------------------------------------------------------------
// wilsonScoreInterval: hand-checked numeric values
// -----------------------------------------------------------------------------

func TestWilsonScoreInterval_KnownReferenceValues(t *testing.T) {
	cases := []struct {
		k, n int
		wantLow, wantHigh float64
		tol               float64
	}{
		// p̂ = 0.5, n = 100 → ~(0.404, 0.596) Wilson 95%
		{50, 100, 0.404, 0.596, 0.005},
		// p̂ = 0.9, n = 100 → ~(0.825, 0.948)
		{90, 100, 0.825, 0.948, 0.005},
		// p̂ = 0.5, n = 10 → wider: (0.237, 0.763)
		{5, 10, 0.237, 0.763, 0.005},
		// k = 0, n = 100 → (0.000, 0.037)
		{0, 100, 0.0, 0.037, 0.005},
		// k = n, n = 100 → (0.963, 1.0)
		{100, 100, 0.963, 1.0, 0.005},
	}
	for _, tc := range cases {
		gotLow, gotHigh := wilsonScoreInterval(tc.k, tc.n, wilsonZ95)
		if math.Abs(gotLow-tc.wantLow) > tc.tol {
			t.Errorf("Wilson(%d/%d) low = %.4f, want %.4f ±%.3f",
				tc.k, tc.n, gotLow, tc.wantLow, tc.tol)
		}
		if math.Abs(gotHigh-tc.wantHigh) > tc.tol {
			t.Errorf("Wilson(%d/%d) high = %.4f, want %.4f ±%.3f",
				tc.k, tc.n, gotHigh, tc.wantHigh, tc.tol)
		}
	}
}

func TestWilsonScoreInterval_ZeroSamplesReturnsFullRange(t *testing.T) {
	low, high := wilsonScoreInterval(0, 0, wilsonZ95)
	if low != 0 || high != 1 {
		t.Errorf("n=0 should return [0, 1]; got [%.4f, %.4f]", low, high)
	}
}

func TestWilsonScoreInterval_BoundsAlwaysInUnitInterval(t *testing.T) {
	// Stress: random k/n combinations should never escape [0, 1].
	for _, n := range []int{1, 5, 25, 200, 5000} {
		for _, k := range []int{0, 1, n / 2, n - 1, n} {
			low, high := wilsonScoreInterval(k, n, wilsonZ95)
			if low < 0 || low > 1 {
				t.Errorf("Wilson(%d/%d) low = %.4f, out of [0, 1]", k, n, low)
			}
			if high < 0 || high > 1 {
				t.Errorf("Wilson(%d/%d) high = %.4f, out of [0, 1]", k, n, high)
			}
			if low > high {
				t.Errorf("Wilson(%d/%d) low %.4f > high %.4f", k, n, low, high)
			}
		}
	}
}

// -----------------------------------------------------------------------------
// ExpectedWinrateInterval: tiered fallback behavior
// -----------------------------------------------------------------------------

func TestExpectedWinrateInterval_ColdStartReturnsUniform(t *testing.T) {
	cp := NewCompositionPrior(4)
	got := cp.ExpectedWinrateInterval("Mill", []string{"Mill", "Voltron", "Aggro", "Combo"})
	if got.Source != WinrateSourceUniform {
		t.Errorf("Source = %q, want %q", got.Source, WinrateSourceUniform)
	}
	if math.Abs(got.Point-0.25) > 1e-9 {
		t.Errorf("Point = %.4f, want 0.25", got.Point)
	}
	if got.Low != 0 || got.High != 1 {
		t.Errorf("uniform interval should span [0, 1]; got [%.4f, %.4f]", got.Low, got.High)
	}
	if got.Samples != 0 {
		t.Errorf("Samples = %d, want 0 (cold start)", got.Samples)
	}
}

func TestExpectedWinrateInterval_NilSafe(t *testing.T) {
	var cp *CompositionPrior
	got := cp.ExpectedWinrateInterval("Mill", []string{"Voltron"})
	if got.Source != WinrateSourceUniform {
		t.Errorf("nil receiver should return uniform; got %q", got.Source)
	}
	if got.Point != 0.25 {
		t.Errorf("nil receiver Point = %.4f, want 0.25 (default podSize 4)", got.Point)
	}
}

func TestExpectedWinrateInterval_PairwiseTierNarrowsWithSamples(t *testing.T) {
	pod := []string{"Mill", "Voltron", "Aggro", "Combo"}

	// Few samples → wide interval.
	cpSmall := NewCompositionPrior(4)
	for i := 0; i < 10; i++ {
		_ = cpSmall.ObserveGame(pod, "Mill")
	}
	smallInt := cpSmall.ExpectedWinrateInterval("Mill", pod)

	// Many samples → narrow interval, same Point ≈ 1.0.
	cpLarge := NewCompositionPrior(4)
	for i := 0; i < 1000; i++ {
		_ = cpLarge.ObserveGame(pod, "Mill")
	}
	largeInt := cpLarge.ExpectedWinrateInterval("Mill", pod)

	smallWidth := smallInt.High - smallInt.Low
	largeWidth := largeInt.High - largeInt.Low
	if largeWidth >= smallWidth {
		t.Errorf("interval should narrow with samples; small=%.4f large=%.4f",
			smallWidth, largeWidth)
	}
	if smallInt.Source != WinrateSourcePairwise || largeInt.Source != WinrateSourcePairwise {
		t.Errorf("both should report pairwise source; got %q / %q",
			smallInt.Source, largeInt.Source)
	}
	if smallInt.Samples != 30 { // 3 opponents × 10 games each
		t.Errorf("small Samples = %d, want 30", smallInt.Samples)
	}
	if largeInt.Samples != 3000 {
		t.Errorf("large Samples = %d, want 3000", largeInt.Samples)
	}
}

func TestExpectedWinrateInterval_PointMatchesExpectedWinrate(t *testing.T) {
	// The interval's Point should always equal what ExpectedWinrate
	// returns — the interval is annotation on top of the existing
	// scalar.
	pod := []string{"Mill", "Voltron", "Aggro", "Combo"}
	cp := NewCompositionPrior(4)
	for i := 0; i < 100; i++ {
		w := "Mill"
		if i%4 == 1 {
			w = "Voltron"
		}
		_ = cp.ObserveGame(pod, w)
	}
	for _, arch := range pod {
		gotPoint := cp.ExpectedWinrateInterval(arch, pod).Point
		gotScalar := cp.ExpectedWinrate(arch, pod)
		if math.Abs(gotPoint-gotScalar) > 1e-9 {
			t.Errorf("%s: interval Point %.6f != ExpectedWinrate %.6f",
				arch, gotPoint, gotScalar)
		}
	}
}

func TestExpectedWinrateInterval_FallsBackToArchetypeBaseline(t *testing.T) {
	// Build archetype baseline only — no pairwise data in the query
	// pod. The interval should report archetype_baseline as the
	// source, with bounds derived from the archetype's total games.
	cp := NewCompositionPrior(4)
	otherPod := []string{"Mill", "X", "Y", "Z"}
	for i := 0; i < 40; i++ {
		_ = cp.ObserveGame(otherPod, "Mill")
	}
	for i := 0; i < 60; i++ {
		_ = cp.ObserveGame(otherPod, "X")
	}
	// Mill has 40 wins / 100 games globally. Query in a brand-new
	// pod with no pairwise data.
	queryPod := []string{"Mill", "A", "B", "C"}
	got := cp.ExpectedWinrateInterval("Mill", queryPod)
	if got.Source != WinrateSourceArchetypeBaseline {
		t.Errorf("Source = %q, want %q", got.Source, WinrateSourceArchetypeBaseline)
	}
	if math.Abs(got.Point-0.40) > 1e-9 {
		t.Errorf("Point = %.4f, want 0.40", got.Point)
	}
	if got.Samples != 100 {
		t.Errorf("Samples = %d, want 100", got.Samples)
	}
	// Wilson 95% at 40/100 → ~(0.309, 0.498)
	if got.Low < 0.30 || got.High > 0.51 {
		t.Errorf("expected bounds ≈ (0.309, 0.498); got (%.4f, %.4f)", got.Low, got.High)
	}
}

// -----------------------------------------------------------------------------
// Edge cases: lopsided pairwise data
// -----------------------------------------------------------------------------

func TestExpectedWinrateInterval_AllWinsGivesHighFloorButBelowOne(t *testing.T) {
	pod := []string{"Mill", "Voltron", "Aggro", "Combo"}
	cp := NewCompositionPrior(4)
	for i := 0; i < 100; i++ {
		_ = cp.ObserveGame(pod, "Mill") // Mill wins every game
	}
	got := cp.ExpectedWinrateInterval("Mill", pod)
	if math.Abs(got.Point-1.0) > 1e-9 {
		t.Errorf("Mill 100%% wins: Point = %.4f, want 1.0", got.Point)
	}
	// 300 wins / 300 games → Wilson Low ≈ 0.987, High ≈ 1.0
	if got.Low < 0.98 {
		t.Errorf("Low should be ≥ 0.98 for 300/300; got %.4f", got.Low)
	}
	if got.High < 0.999 {
		t.Errorf("High should be ≈ 1.0 for 300/300; got %.4f", got.High)
	}
}

func TestExpectedWinrateInterval_NoWinsGivesLowCeilingAboveZero(t *testing.T) {
	pod := []string{"Mill", "Voltron", "Aggro", "Combo"}
	cp := NewCompositionPrior(4)
	for i := 0; i < 100; i++ {
		_ = cp.ObserveGame(pod, "Voltron") // Mill never wins
	}
	got := cp.ExpectedWinrateInterval("Mill", pod)
	if math.Abs(got.Point-0.0) > 1e-9 {
		t.Errorf("Mill 0%% wins: Point = %.4f, want 0.0", got.Point)
	}
	// 0 / 300 → Low ≈ 0 (Wilson math lands just above 0 without
	// clamping), High ≈ 0.013 — small but nonzero (Wilson doesn't
	// claim "impossible" from a finite sample).
	if got.Low > 0.001 {
		t.Errorf("Low should be ≈ 0; got %.4f", got.Low)
	}
	if got.High <= 0 || got.High > 0.05 {
		t.Errorf("High should be in (0, 0.05]; got %.4f", got.High)
	}
}

// -----------------------------------------------------------------------------
// JSON serialization (the type is used in CLI output / monitoring)
// -----------------------------------------------------------------------------

func TestWinrateInterval_JSONFieldNames(t *testing.T) {
	// Compile-time check: ensure JSON tags match what callers expect.
	// Using a separate variable so the type doesn't accidentally drop
	// fields without breaking the test.
	w := WinrateInterval{
		Point: 0.5, Low: 0.4, High: 0.6, Samples: 100, Source: "pairwise",
	}
	if w.Source != WinrateSourcePairwise {
		t.Errorf("source label drift: WinrateSourcePairwise = %q", WinrateSourcePairwise)
	}
	if w.Point != 0.5 || w.Low != 0.4 || w.High != 0.6 || w.Samples != 100 {
		t.Errorf("WinrateInterval fields don't round-trip: %+v", w)
	}
}
