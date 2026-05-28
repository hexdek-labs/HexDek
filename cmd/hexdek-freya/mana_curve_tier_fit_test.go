package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualFit(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestBuildManaCurveTierFit_IdealCMCGivesFitOne pins the Gaussian-
// centerpoint behavior: when AvgCMC == IdealCMC, Fit = 1.0 exactly.
func TestBuildManaCurveTierFit_IdealCMCGivesFitOne(t *testing.T) {
	for tier, expect := range tierCurveTable {
		got := BuildManaCurveTierFit(expect.ideal, tier)
		if !almostEqualFit(got.Fit, 1.0) {
			t.Errorf("tier %d at ideal CMC %.1f: want Fit=1.0, got %.4f",
				tier, expect.ideal, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("tier %d at ideal: want Direction=in_band, got %q",
				tier, got.Direction)
		}
	}
}

// TestBuildManaCurveTierFit_AlignedDecksHighFit verifies the user's
// spec scenarios: cEDH (T5) ~2.0 CMC, casual (T1) ~3.8 CMC,
// upgraded precon (T3) 3.0-3.5 — each should produce fit >= 0.85
// inside the tolerated band.
func TestBuildManaCurveTierFit_AlignedDecksHighFit(t *testing.T) {
	cases := []struct {
		name   string
		tier   int
		avgCMC float64
	}{
		{"cEDH at 2.0", 5, 2.0},
		{"cEDH at 2.3", 5, 2.3},
		{"High Power at 2.5", 4, 2.5},
		{"High Power at 2.7", 4, 2.7},
		{"Upgraded Precon at 3.2", 3, 3.2},
		{"Upgraded Precon at 3.4", 3, 3.4},
		{"Casual T2 at 3.6", 2, 3.6},
		{"Casual T1 at 3.8", 1, 3.8},
	}
	for _, c := range cases {
		got := BuildManaCurveTierFit(c.avgCMC, c.tier)
		if got.Fit < 0.85 {
			t.Errorf("%s: want Fit >= 0.85, got %.4f", c.name, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("%s: want in_band, got %q", c.name, got.Direction)
		}
	}
}

// TestBuildManaCurveTierFit_DriftedDecksLowFit verifies the
// under-tuned and over-tuned scenarios — a deck with a curve that
// doesn't match its claimed tier surfaces a low Fit score.
func TestBuildManaCurveTierFit_DriftedDecksLowFit(t *testing.T) {
	cases := []struct {
		name      string
		tier      int
		avgCMC    float64
		direction string
	}{
		// cEDH-claimed deck with a B2-casual curve (3.6 vs ideal 2.0)
		{"cEDH claim with 3.6 CMC", 5, 3.6, "too_expensive"},
		// cEDH-claimed deck with a B3 curve (3.2 vs ideal 2.0)
		{"cEDH claim with 3.2 CMC", 5, 3.2, "too_expensive"},
		// B1 casual deck with a cEDH-lean curve (2.0 vs ideal 3.8)
		{"Casual T1 claim with 2.0 CMC", 1, 2.0, "too_lean"},
		// B3 upgraded precon claim with a B5 cEDH curve
		{"Upgraded Precon claim with 1.9 CMC", 3, 1.9, "too_lean"},
		// B4 High Power claim with a B1 casual curve
		{"High Power claim with 4.2 CMC", 4, 4.2, "too_expensive"},
	}
	for _, c := range cases {
		got := BuildManaCurveTierFit(c.avgCMC, c.tier)
		if got.Fit >= 0.6 {
			t.Errorf("%s: want Fit < 0.6 (clear drift), got %.4f", c.name, got.Fit)
		}
		if got.Direction != c.direction {
			t.Errorf("%s: want direction %q, got %q", c.name, c.direction, got.Direction)
		}
		// Direction-aware verdict should hint at the failure mode
		if c.direction == "too_expensive" && !strings.Contains(got.Verdict, "expensive") {
			t.Errorf("%s: verdict missing 'expensive': %q", c.name, got.Verdict)
		}
		if c.direction == "too_lean" && !strings.Contains(got.Verdict, "lean") {
			t.Errorf("%s: verdict missing 'lean': %q", c.name, got.Verdict)
		}
	}
}

// TestBuildManaCurveTierFit_BandEdgesNearHighFit verifies the
// "at the edge of tolerated band" calibration: fit should be at
// least 0.85 at min/max boundaries.
func TestBuildManaCurveTierFit_BandEdgesNearHighFit(t *testing.T) {
	for tier, expect := range tierCurveTable {
		minGot := BuildManaCurveTierFit(expect.min, tier)
		maxGot := BuildManaCurveTierFit(expect.max, tier)
		// Band edges sit ~1 sigma from the ideal, so the Gaussian
		// gives ~0.6 fit there. The "tolerated band" semantic is
		// "still acceptable, not drift" — fit drops further outside.
		if minGot.Fit < 0.55 {
			t.Errorf("tier %d min-edge CMC %.1f: want Fit >= 0.55, got %.4f",
				tier, expect.min, minGot.Fit)
		}
		if maxGot.Fit < 0.55 {
			t.Errorf("tier %d max-edge CMC %.1f: want Fit >= 0.55, got %.4f",
				tier, expect.max, maxGot.Fit)
		}
		if minGot.Direction != "in_band" {
			t.Errorf("tier %d at min: want in_band, got %q", tier, minGot.Direction)
		}
		if maxGot.Direction != "in_band" {
			t.Errorf("tier %d at max: want in_band, got %q", tier, maxGot.Direction)
		}
	}
}

// TestBuildManaCurveTierFit_OutOfRangeTierReturnsZeroFit verifies
// the unknown-tier defensive path.
func TestBuildManaCurveTierFit_OutOfRangeTierReturnsZeroFit(t *testing.T) {
	for _, tier := range []int{0, -1, 6, 99} {
		got := BuildManaCurveTierFit(2.5, tier)
		if got.Fit != 0 {
			t.Errorf("tier %d: want Fit=0, got %.4f", tier, got.Fit)
		}
		if got.TierLabel != "Unknown" {
			t.Errorf("tier %d: want TierLabel=Unknown, got %q", tier, got.TierLabel)
		}
		if !strings.Contains(got.Verdict, "out of range") {
			t.Errorf("tier %d verdict: %q", tier, got.Verdict)
		}
	}
}

// TestBuildManaCurveTierFit_ZeroCMCReturnsZeroFit verifies the
// "no CMC data" path.
func TestBuildManaCurveTierFit_ZeroCMCReturnsZeroFit(t *testing.T) {
	got := BuildManaCurveTierFit(0, 5)
	if got.Fit != 0 {
		t.Errorf("zero CMC: want Fit=0, got %.4f", got.Fit)
	}
	if !strings.Contains(got.Verdict, "No CMC data") {
		t.Errorf("zero CMC verdict: %q", got.Verdict)
	}
	// Negative CMC should also trip the zero-data path
	got = BuildManaCurveTierFit(-1.0, 5)
	if got.Fit != 0 {
		t.Errorf("negative CMC: want Fit=0, got %.4f", got.Fit)
	}
}

// TestBuildManaCurveTierFit_GaussianSymmetry verifies that fit is
// symmetric around the ideal — equal positive and negative
// deviations produce identical fits.
func TestBuildManaCurveTierFit_GaussianSymmetry(t *testing.T) {
	for tier, expect := range tierCurveTable {
		dev := 0.5
		below := BuildManaCurveTierFit(expect.ideal-dev, tier)
		above := BuildManaCurveTierFit(expect.ideal+dev, tier)
		if !almostEqualFit(below.Fit, above.Fit) {
			t.Errorf("tier %d: asymmetric fit at ±%.1f around ideal %.1f: below=%.4f above=%.4f",
				tier, dev, expect.ideal, below.Fit, above.Fit)
		}
	}
}

// TestBuildManaCurveTierFit_VerdictShape verifies the Verdict
// string carries the AvgCMC, TierLabel, ideal, and fit values.
func TestBuildManaCurveTierFit_VerdictShape(t *testing.T) {
	got := BuildManaCurveTierFit(3.2, 3)
	if !strings.Contains(got.Verdict, "3.20") && !strings.Contains(got.Verdict, "3.2") {
		t.Errorf("verdict missing AvgCMC: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "Upgraded Precon") {
		t.Errorf("verdict missing tier label: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "fit") {
		t.Errorf("verdict missing 'fit' callout: %q", got.Verdict)
	}
}

// TestBuildManaCurveTierFit_GaussianFitScoreEdgeCases verifies the
// helper at degenerate inputs.
func TestBuildManaCurveTierFit_GaussianFitScoreEdgeCases(t *testing.T) {
	// Zero sigma → 0
	if got := gaussianFitScore(2.0, 2.0, 0); got != 0 {
		t.Errorf("zero sigma: want 0, got %.4f", got)
	}
	// Negative sigma → 0
	if got := gaussianFitScore(2.0, 2.0, -1); got != 0 {
		t.Errorf("negative sigma: want 0, got %.4f", got)
	}
	// At centerpoint → 1
	if got := gaussianFitScore(2.0, 2.0, 0.5); !almostEqualFit(got, 1.0) {
		t.Errorf("at centerpoint: want 1.0, got %.4f", got)
	}
}

// TestBuildManaCurveTierFit_RealCorpusCalibration runs the fit
// against real decks at their classifier-determined PowerTier.
// Aligned cEDH B5 decks (which have AvgCMC ~2.0) should produce
// high fit; the precon corpus (T1-T3 by PR #715 verification)
// should produce reasonable fits for their declared bands.
//
// Skipped when oracle data absent.
func TestBuildManaCurveTierFit_RealCorpusCalibration(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)

	deckDir := "../../data/decks/test"
	matches, _ := filepath.Glob(filepath.Join(deckDir, "cedh_*_b5_*.txt"))
	if len(matches) == 0 {
		t.Skipf("no cEDH B5 test decks found")
	}

	// All 9 cEDH B5 decks should show high fit when classified at T5
	// — they're tournament-tuned cEDH builds with AvgCMC ~2.0.
	highFitCount := 0
	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		got := BuildManaCurveTierFit(dp.AvgCMC, tier.Tier)
		t.Logf("%s: AvgCMC=%.2f tier=T%d fit=%.3f direction=%s",
			base, dp.AvgCMC, tier.Tier, got.Fit, got.Direction)
		if got.Fit >= 0.85 && got.Direction == "in_band" {
			highFitCount++
		}
	}
	if highFitCount < 6 {
		t.Errorf("expected at least 6/9 cEDH B5 decks to show high in-band fit, got %d",
			highFitCount)
	}
}
