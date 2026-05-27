package main

import (
	"strings"
	"testing"
)

// cmc_distribution_r60_test.go — pins the r60 CMC-distribution health
// check. Two thresholds, both calibrated via opening-hand hypergeometric
// math (deck of 99, opener of 7):
//
//   - OneDropMax = 15: with 15 one-drops, ≈21% of openers contain ≥2
//     one-drops — beyond this density the curve dilutes mid-game tempo.
//   - TwoDropMin = 6: with 6 two-drops, ≈64% of openers contain zero
//     two-drops — below this floor turn-2 plays whiff regularly.
//
// Tests pin: the thresholds themselves (defend against accidental
// retuning), both warning paths, the no-fire boundary on each side,
// the dp.OneDropCount/TwoDropCount field population, defensive skip on
// empty reports, openingHandWhiffPct closed-form math against known
// hypergeometric values, idempotency.

// -----------------------------------------------------------------------------
// Thresholds — pin the calibration values so a future "tune" doesn't
// silently retune the warning floor without a deliberate test update.
// -----------------------------------------------------------------------------

func TestCMCDistribution_DefaultThresholds(t *testing.T) {
	if defaultCMCDistribution.OneDropMax != 15 {
		t.Errorf("OneDropMax calibrated to 15, got %d — retune requires test update + rationale", defaultCMCDistribution.OneDropMax)
	}
	if defaultCMCDistribution.TwoDropMin != 6 {
		t.Errorf("TwoDropMin calibrated to 6, got %d — retune requires test update + rationale", defaultCMCDistribution.TwoDropMin)
	}
}

// -----------------------------------------------------------------------------
// openingHandWhiffPct — hypergeometric closed-form
// -----------------------------------------------------------------------------

func TestOpeningHandWhiff_KnownCases(t *testing.T) {
	// Hand-calculated cases against (N-K choose n)/(N choose n) for a
	// deck of 99 / opener of 7.
	cases := []struct {
		name       string
		k, deck, n int
		want       float64
		tol        float64
	}{
		// 6 two-drops in 99 cards, opener of 7: P(X=0) ≈ 64%.
		{"6 two-drops in 99", 6, 99, 7, 64.0, 1.5},
		// 8 two-drops: ≈55%.
		{"8 two-drops in 99", 8, 99, 7, 55.0, 2.0},
		// 12 two-drops: ≈38%.
		{"12 two-drops in 99", 12, 99, 7, 38.0, 2.0},
		// Sol Ring lookalike: 1 copy of a card in 99 → P(no copy in 7) ≈ 92.9%.
		{"1 copy in 99", 1, 99, 7, 92.93, 0.1},
		// 0-of: 100% whiff.
		{"k=0 → 100% whiff", 0, 99, 7, 100, 0.001},
		// k > deck: malformed input, returns 0 (graceful).
		{"k > deck", 200, 99, 7, 0, 0.001},
		// n > deck: malformed.
		{"n > deck", 5, 4, 7, 0, 0.001},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := openingHandWhiffPct(c.k, c.deck, c.n)
			diff := got - c.want
			if diff < 0 {
				diff = -diff
			}
			if diff > c.tol {
				t.Errorf("openingHandWhiffPct(k=%d,N=%d,n=%d) = %.2f, want %.2f (tol %.2f)",
					c.k, c.deck, c.n, got, c.want, c.tol)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// computeCMCDistributionHealth — happy path and edges
// -----------------------------------------------------------------------------

func newReportWithCurve(oneDrops, twoDrops, otherNonland, lands int) *FreyaReport {
	r := &FreyaReport{
		TotalCards:   oneDrops + twoDrops + otherNonland + lands,
		LandCount:    lands,
		NonlandCount: oneDrops + twoDrops + otherNonland,
	}
	r.ManaCurve[1] = oneDrops
	r.ManaCurve[2] = twoDrops
	// Park the "other" cards at CMC 4 so they don't trip the existing
	// bottom-light / top-heavy warnings tangled in unrelated logic.
	r.ManaCurve[4] = otherNonland
	return r
}

func TestComputeCMCDistribution_OneDropHeavy_Fires(t *testing.T) {
	report := newReportWithCurve(16, 10, 36, 37) // 99 total, 16 one-drops
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if dp.OneDropCount != 16 || dp.TwoDropCount != 10 {
		t.Errorf("count fields not populated: got OneDrop=%d TwoDrop=%d, want 16/10", dp.OneDropCount, dp.TwoDropCount)
	}
	if !containsSubstring(report.CurveWarnings, "1-drop heavy: 16") {
		t.Errorf("expected 1-drop-heavy warning, got %v", report.CurveWarnings)
	}
	if containsSubstring(report.CurveWarnings, "2-drop starved") {
		t.Errorf("must not fire 2-drop warning with 10 two-drops, got %v", report.CurveWarnings)
	}
}

func TestComputeCMCDistribution_OneDropAtBoundary_NoFire(t *testing.T) {
	report := newReportWithCurve(15, 10, 37, 37) // exactly at threshold
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if containsSubstring(report.CurveWarnings, "1-drop heavy") {
		t.Errorf("15 one-drops sits AT the threshold (>15 fires); got false positive: %v", report.CurveWarnings)
	}
}

func TestComputeCMCDistribution_TwoDropStarved_Fires(t *testing.T) {
	report := newReportWithCurve(8, 5, 49, 37) // 99 total, 5 two-drops
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if dp.TwoDropCount != 5 {
		t.Errorf("TwoDropCount not populated, got %d", dp.TwoDropCount)
	}
	if !containsSubstring(report.CurveWarnings, "2-drop starved: only 5") {
		t.Errorf("expected 2-drop-starved warning, got %v", report.CurveWarnings)
	}
	// Warning must include the computed whiff percentage.
	if !containsSubstring(report.CurveWarnings, "% of openers") {
		t.Errorf("warning text must include the whiff percentage, got %v", report.CurveWarnings)
	}
}

func TestComputeCMCDistribution_TwoDropAtBoundary_NoFire(t *testing.T) {
	report := newReportWithCurve(8, 6, 48, 37) // exactly at threshold
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if containsSubstring(report.CurveWarnings, "2-drop starved") {
		t.Errorf("6 two-drops sits AT the threshold (<6 fires); got false positive: %v", report.CurveWarnings)
	}
}

func TestComputeCMCDistribution_BothViolations_FireBoth(t *testing.T) {
	report := newReportWithCurve(20, 3, 39, 37) // 99 total, both violations
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if !containsSubstring(report.CurveWarnings, "1-drop heavy: 20") {
		t.Errorf("missing 1-drop warning, got %v", report.CurveWarnings)
	}
	if !containsSubstring(report.CurveWarnings, "2-drop starved: only 3") {
		t.Errorf("missing 2-drop warning, got %v", report.CurveWarnings)
	}
	if len(report.CurveWarnings) != 2 {
		t.Errorf("expected exactly 2 warnings, got %d (%v)", len(report.CurveWarnings), report.CurveWarnings)
	}
}

func TestComputeCMCDistribution_HealthyEDH_NoWarn(t *testing.T) {
	// Typical EDH curve: 4 one-drops, 11 two-drops, 47 other (rest of curve),
	// 37 lands. Nothing fires.
	report := newReportWithCurve(4, 11, 47, 37)
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)

	if len(report.CurveWarnings) != 0 {
		t.Errorf("healthy EDH curve must not warn, got %v", report.CurveWarnings)
	}
	if dp.OneDropCount != 4 || dp.TwoDropCount != 11 {
		t.Errorf("count fields should populate even when no warning fires; got %d/%d", dp.OneDropCount, dp.TwoDropCount)
	}
}

func TestComputeCMCDistribution_EmptyReport_NoOp(t *testing.T) {
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, &FreyaReport{}) // NonlandCount == 0
	if dp.OneDropCount != 0 || dp.TwoDropCount != 0 {
		t.Errorf("empty report must leave counts at 0")
	}
}

func TestComputeCMCDistribution_NilDp_NoCrash(t *testing.T) {
	computeCMCDistributionHealth(nil, &FreyaReport{NonlandCount: 60})
}

func TestComputeCMCDistribution_NilReport_NoCrash(t *testing.T) {
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, nil)
}

func TestComputeCMCDistribution_Idempotent(t *testing.T) {
	report := newReportWithCurve(20, 3, 39, 37)
	dp := &DeckProfile{}
	computeCMCDistributionHealth(dp, report)
	first := len(report.CurveWarnings)
	computeCMCDistributionHealth(dp, report)
	if len(report.CurveWarnings) != first {
		t.Errorf("idempotent call appended duplicates: %d -> %d (%v)", first, len(report.CurveWarnings), report.CurveWarnings)
	}
}

// -----------------------------------------------------------------------------
// deriveWeaknesses — confirm the brief callout flows through
// -----------------------------------------------------------------------------

func TestDeriveWeaknesses_OneDropHeavy_IncludesCallout(t *testing.T) {
	dp := &DeckProfile{
		OneDropCount: 18,
		TwoDropCount: 10,
		WinLineCount: 2,
	}
	weakness := deriveWeaknesses(&FreyaReport{TotalCards: 99}, dp)
	if !containsSubstring(weakness, "1-drop heavy (18") {
		t.Errorf("deriveWeaknesses must surface 1-drop callout, got %v", weakness)
	}
}

func TestDeriveWeaknesses_TwoDropStarved_IncludesCallout(t *testing.T) {
	dp := &DeckProfile{
		OneDropCount: 8,
		TwoDropCount: 4,
		WinLineCount: 2,
	}
	weakness := deriveWeaknesses(&FreyaReport{TotalCards: 99}, dp)
	if !containsSubstring(weakness, "2-drop starved (4") {
		t.Errorf("deriveWeaknesses must surface 2-drop callout, got %v", weakness)
	}
}

func TestDeriveWeaknesses_HealthyCurve_NoCallout(t *testing.T) {
	dp := &DeckProfile{
		OneDropCount: 5,
		TwoDropCount: 10,
		WinLineCount: 2,
	}
	weakness := deriveWeaknesses(&FreyaReport{TotalCards: 99}, dp)
	if containsSubstring(weakness, "1-drop heavy") || containsSubstring(weakness, "2-drop starved") {
		t.Errorf("healthy curve must not produce CMC distribution callouts, got %v", weakness)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func containsSubstring(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
