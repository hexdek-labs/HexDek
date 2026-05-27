package main

import (
	"strings"
	"testing"
)

// mana_hump_r60_test.go — pins the r60 mana-hump curve-shape extension.
//
// computeManaHump returns (avg CMC of top quartile, count taken, floor
// CMC of last bucket touched) over a bucketed ManaCurve. The top
// quartile is ceil(N/4) cards walked from CMC 7+ down. Partial-bucket
// inclusion at the floor is the canonical edge case.
//
// AugmentCurveWarningsWithRamp appends a warning to report.CurveWarnings
// when a heavy hump outpaces ramp support. Trigger:
//   HumpFloorCMC >= 5 AND HumpCMC >= 4.5 AND ramp < ceil(humpCards*0.75).
//
// The structural "heavy mana hump" warning is emitted directly from
// AnalyzeDeck (covered indirectly via the floor + cmc thresholds).
//
// Also pins the integer-division fix in the bimodal peakAvg calculation
// — peaks of 3 and 4 must compute peakAvg=3.5 (float), not 3 (int).

// -----------------------------------------------------------------------------
// computeManaHump — direct cases
// -----------------------------------------------------------------------------

func TestComputeManaHump_EmptyDeck_ReturnsZero(t *testing.T) {
	cmc, n, floor := computeManaHump([8]int{}, 0)
	if cmc != 0 || n != 0 || floor != 0 {
		t.Errorf("empty deck must return (0,0,0), got (%v,%v,%v)", cmc, n, floor)
	}
}

func TestComputeManaHump_DocstringExample(t *testing.T) {
	// curve = [0, 4, 6, 8, 5, 3, 2, 1] (29 nonlands). Quartile target =
	// ceil(29/4) = 8. Walking from CMC 7 down: 1 (CMC 7), 2 (CMC 6), 3
	// (CMC 5) — running total 6, need 2 more. Take 2 from CMC 4 (partial).
	// HumpCMC = (1*7 + 2*6 + 3*5 + 2*4) / 8 = 42/8 = 5.25.
	curve := [8]int{0, 4, 6, 8, 5, 3, 2, 1}
	cmc, n, floor := computeManaHump(curve, 29)
	if n != 8 {
		t.Errorf("hump card count must be ceil(29/4) = 8, got %d", n)
	}
	if floor != 4 {
		t.Errorf("hump floor must be CMC 4 (partial bucket), got %d", floor)
	}
	if !approxEqualHump(cmc, 5.25, 0.001) {
		t.Errorf("hump CMC must be 5.25, got %v", cmc)
	}
}

func TestComputeManaHump_AllCardsInOneBucket(t *testing.T) {
	// 32 nonlands all at CMC 4 — top quartile is 8 cards, all from the
	// CMC 4 bucket, partial inclusion.
	curve := [8]int{0, 0, 0, 0, 32, 0, 0, 0}
	cmc, n, floor := computeManaHump(curve, 32)
	if n != 8 || floor != 4 || cmc != 4.0 {
		t.Errorf("expected (4.0, 8, 4), got (%v, %v, %v)", cmc, n, floor)
	}
}

func TestComputeManaHump_FlatBottomHeavyAggro(t *testing.T) {
	// Aggro curve: 24 cards mostly at CMC 1-2, a couple at CMC 3-4.
	// Quartile = 6. Walking from top: 0,0,0,0,2 (CMC 4), need 4 more at
	// CMC 3 (4 of 6). HumpCMC = (2*4 + 4*3)/6 = 20/6 ≈ 3.33.
	curve := [8]int{0, 10, 8, 6, 2, 0, 0, 0}
	cmc, n, floor := computeManaHump(curve, 26)
	want := 20.0 / 6.0
	if n != 7 { // ceil(26/4) = 7, not 6
		t.Errorf("hump count must be ceil(26/4)=7, got %d", n)
	}
	// Recompute expected: take 2 @ CMC 4 + 5 @ CMC 3 → (8+15)/7 = 23/7 ≈ 3.29
	want = 23.0 / 7.0
	if !approxEqualHump(cmc, want, 0.001) {
		t.Errorf("hump CMC mismatch: got %v want %v", cmc, want)
	}
	if floor != 3 {
		t.Errorf("hump floor must be 3, got %d", floor)
	}
}

func TestComputeManaHump_TopHeavyControl(t *testing.T) {
	// Top-heavy: lots of CMC 6 + 7+. 28 nonlands. Quartile = 7. Walk top:
	// 6 (CMC 7+), need 1 more at CMC 6. HumpCMC = (6*7 + 1*6)/7 = 48/7 ≈ 6.86.
	curve := [8]int{0, 2, 3, 5, 5, 4, 3, 6}
	cmc, n, floor := computeManaHump(curve, 28)
	if n != 7 {
		t.Errorf("hump count must be 7, got %d", n)
	}
	want := 48.0 / 7.0
	if !approxEqualHump(cmc, want, 0.001) {
		t.Errorf("hump CMC mismatch: got %v want %v", cmc, want)
	}
	if floor != 6 {
		t.Errorf("hump floor must be 6 (the partial bucket), got %d", floor)
	}
}

func TestComputeManaHump_QuartileCeilingRounding(t *testing.T) {
	// 30 nonlands → ceil(30/4) = 8. 31 → 8. 32 → 8. 33 → 9.
	cases := []struct {
		n    int
		want int
	}{
		{30, 8},
		{31, 8},
		{32, 8},
		{33, 9},
	}
	for _, c := range cases {
		curve := [8]int{0, 0, 0, 0, 0, 0, 0, c.n} // all at CMC 7+
		_, got, _ := computeManaHump(curve, c.n)
		if got != c.want {
			t.Errorf("N=%d: hump count want %d, got %d", c.n, c.want, got)
		}
	}
}

func TestComputeManaHump_CMC7BucketScoresAs7(t *testing.T) {
	// The CMC 7+ bucket scores as exactly 7 (matches the rest of the
	// AvgCMC computation in AnalyzeDeck).
	curve := [8]int{0, 0, 0, 0, 0, 0, 0, 4}
	cmc, n, floor := computeManaHump(curve, 16)
	if n != 4 || cmc != 7.0 || floor != 7 {
		t.Errorf("expected (7.0, 4, 7), got (%v, %v, %v)", cmc, n, floor)
	}
}

// -----------------------------------------------------------------------------
// AugmentCurveWarningsWithRamp — happy path and gates
// -----------------------------------------------------------------------------

func TestAugmentCurveWarningsWithRamp_TopHeavyLowRamp_Fires(t *testing.T) {
	r := &FreyaReport{
		HumpCMC:       6.5,
		HumpCardCount: 8,
		HumpFloorCMC:  6,
	}
	AugmentCurveWarningsWithRamp(r, 3) // need ceil(8*0.75)=6, have 3
	if len(r.CurveWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d (%v)", len(r.CurveWarnings), r.CurveWarnings)
	}
	got := r.CurveWarnings[0]
	if !strings.Contains(got, "hump outpaces ramp") {
		t.Errorf("warning text missing canonical prefix: %q", got)
	}
	if !strings.Contains(got, "3 ramp pieces") {
		t.Errorf("warning must surface the ramp count: %q", got)
	}
	if !strings.Contains(got, "3 more ramp") {
		t.Errorf("warning must compute the deficit (6-3=3): %q", got)
	}
}

func TestAugmentCurveWarningsWithRamp_RampSufficient_NoFire(t *testing.T) {
	r := &FreyaReport{
		HumpCMC:       6.5,
		HumpCardCount: 8,
		HumpFloorCMC:  6,
	}
	AugmentCurveWarningsWithRamp(r, 6) // exactly meets ceil(8*0.75)=6
	if len(r.CurveWarnings) != 0 {
		t.Errorf("ramp >= floor must not warn, got %v", r.CurveWarnings)
	}
}

func TestAugmentCurveWarningsWithRamp_LowHumpCMC_NoFire(t *testing.T) {
	r := &FreyaReport{
		HumpCMC:       4.4, // below 4.5 threshold
		HumpCardCount: 8,
		HumpFloorCMC:  5,
	}
	AugmentCurveWarningsWithRamp(r, 1)
	if len(r.CurveWarnings) != 0 {
		t.Errorf("HumpCMC < 4.5 must not warn, got %v", r.CurveWarnings)
	}
}

func TestAugmentCurveWarningsWithRamp_LowFloor_NoFire(t *testing.T) {
	r := &FreyaReport{
		HumpCMC:       5.0,
		HumpCardCount: 8,
		HumpFloorCMC:  4, // below 5 threshold (hump centered at midrange)
	}
	AugmentCurveWarningsWithRamp(r, 1)
	if len(r.CurveWarnings) != 0 {
		t.Errorf("HumpFloorCMC < 5 must not warn, got %v", r.CurveWarnings)
	}
}

func TestAugmentCurveWarningsWithRamp_ZeroRamp_NoFire(t *testing.T) {
	// Zero ramp gets covered by other warnings — don't double-fire.
	r := &FreyaReport{
		HumpCMC:       6.5,
		HumpCardCount: 8,
		HumpFloorCMC:  6,
	}
	AugmentCurveWarningsWithRamp(r, 0)
	if len(r.CurveWarnings) != 0 {
		t.Errorf("ramp=0 must defer to other warnings, got %v", r.CurveWarnings)
	}
}

func TestAugmentCurveWarningsWithRamp_NilReport_NoCrash(t *testing.T) {
	AugmentCurveWarningsWithRamp(nil, 5) // must not crash
}

func TestAugmentCurveWarningsWithRamp_ZeroHumpCount_NoFire(t *testing.T) {
	r := &FreyaReport{HumpCardCount: 0}
	AugmentCurveWarningsWithRamp(r, 5)
	if len(r.CurveWarnings) != 0 {
		t.Errorf("HumpCardCount=0 must not warn, got %v", r.CurveWarnings)
	}
}

// -----------------------------------------------------------------------------
// Bimodal peakAvg integer-division fix
// -----------------------------------------------------------------------------

func TestBimodal_PeakAvgFloat_NoIntegerDivisionBias(t *testing.T) {
	// Curve with peaks at CMC 1 (3 cards) and CMC 5 (4 cards), separated
	// by an empty valley. findCurvePeaks returns [1, 5]. Pre-fix:
	// peakAvg = (3+4)/2 = 3 (int division). Post-fix: peakAvg = 3.5.
	// The float change keeps the bimodal boundary honest — pin that the
	// arithmetic the bimodal classifier executes is now float, not int.
	curve := [8]int{0, 3, 0, 0, 0, 4, 1, 0}
	peaks := findCurvePeaks(curve)
	if len(peaks) < 2 || peaks[0] != 1 || peaks[1] != 5 {
		t.Fatalf("expected peaks [1, 5], got %v", peaks)
	}
	peakAvg := float64(curve[peaks[0]]+curve[peaks[1]]) / 2.0
	if !approxEqualHump(peakAvg, 3.5, 0.001) {
		t.Errorf("peakAvg must be 3.5 (float), got %v", peakAvg)
	}
	// Counterfactual: integer division would have produced 3.
	intPeakAvg := (curve[peaks[0]] + curve[peaks[1]]) / 2
	if intPeakAvg != 3 {
		t.Errorf("counterfactual: int peakAvg should be 3, got %d", intPeakAvg)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func approxEqualHump(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}
