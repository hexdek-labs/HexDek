package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualTDF(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestBuildTutorDensityTierFit_IdealDensityGivesFitOne pins the
// Gaussian centerpoint: when ActualDensity == IdealDensity (within
// integer-tutor rounding), Fit ≈ 1.0.
func TestBuildTutorDensityTierFit_IdealDensityGivesFitOne(t *testing.T) {
	for tier, expect := range tierTutorTable {
		// Pick a tutor count that lands exactly on the ideal at
		// totalCards=100 (so densities round cleanly).
		tutors := int(expect.ideal * 100)
		got := BuildTutorDensityTierFit(tutors, 100, tier)
		if !almostEqualTDF(got.Fit, 1.0) {
			t.Errorf("tier %d at ideal density %.2f (%d/100): want Fit=1.0, got %.4f",
				tier, expect.ideal, tutors, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("tier %d at ideal: want in_band, got %q", tier, got.Direction)
		}
	}
}

// TestBuildTutorDensityTierFit_AlignedDecksHighFit verifies the
// user's spec scenarios: cEDH (T5) ≥10% tutors, B3 upgraded 3-6%,
// B1 casual ≤2% — each within band should produce Fit ≥ 0.85.
func TestBuildTutorDensityTierFit_AlignedDecksHighFit(t *testing.T) {
	cases := []struct {
		name        string
		tier        int
		tutors      int
		totalCards  int
	}{
		{"cEDH at 12% (12/99)", 5, 12, 99},
		{"cEDH at 13% (13/99)", 5, 13, 99},
		{"High Power at 7% (7/99)", 4, 7, 99},
		{"Upgraded Precon at 4% (4/99)", 3, 4, 99},
		{"Upgraded Precon at 5% (5/99)", 3, 5, 99},
		{"Casual T2 at 2% (2/99)", 2, 2, 99},
		{"Casual T1 at 1% (1/99)", 1, 1, 99},
		{"Casual T1 at 0% (0/99)", 1, 0, 99},
	}
	for _, c := range cases {
		got := BuildTutorDensityTierFit(c.tutors, c.totalCards, c.tier)
		if got.Fit < 0.85 {
			t.Errorf("%s: want Fit >= 0.85, got %.4f (density=%.3f)",
				c.name, got.Fit, got.ActualDensity)
		}
		if got.Direction != "in_band" {
			t.Errorf("%s: want in_band, got %q", c.name, got.Direction)
		}
	}
}

// TestBuildTutorDensityTierFit_UnderTunedCEDH verifies the flagship
// spec scenario: a cEDH-claimed deck with only 2 tutors (2% density)
// surfaces low fit + too_sparse direction + verdict mentioning
// "under-tuned" / "consistency too low".
func TestBuildTutorDensityTierFit_UnderTunedCEDH(t *testing.T) {
	got := BuildTutorDensityTierFit(2, 99, 5)
	if got.Fit >= 0.4 {
		t.Errorf("2-tutor cEDH deck: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_sparse" {
		t.Errorf("direction: want too_sparse, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "under-tuned") {
		t.Errorf("verdict missing 'under-tuned': %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "consistency too low") {
		t.Errorf("verdict missing 'consistency too low': %q", got.Verdict)
	}
}

// TestBuildTutorDensityTierFit_OverTunedCasual verifies the
// pubstomp scenario: a casual-claimed (T1) deck with 15% tutors
// surfaces too_dense + verdict mentioning pubstomp risk.
func TestBuildTutorDensityTierFit_OverTunedCasual(t *testing.T) {
	got := BuildTutorDensityTierFit(15, 99, 1)
	if got.Fit >= 0.4 {
		t.Errorf("15-tutor T1-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_dense" {
		t.Errorf("direction: want too_dense, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "pubstomp") {
		t.Errorf("verdict missing 'pubstomp': %q", got.Verdict)
	}
}

// TestBuildTutorDensityTierFit_DriftedDecks tests additional
// drift scenarios beyond the two flagship cases.
func TestBuildTutorDensityTierFit_DriftedDecks(t *testing.T) {
	cases := []struct {
		name      string
		tier      int
		tutors    int
		totalCards int
		direction string
		maxFit    float64
	}{
		// cEDH with B3-typical tutor count (4 tutors = 4%)
		{"T5 with 4 tutors", 5, 4, 99, "too_sparse", 0.6},
		// T2 casual with B4-typical tutor count (7 tutors = 7%)
		{"T2 with 7 tutors", 2, 7, 99, "too_dense", 0.6},
		// T4 with T1-casual tutor count (1 tutor)
		{"T4 with 1 tutor", 4, 1, 99, "too_sparse", 0.6},
	}
	for _, c := range cases {
		got := BuildTutorDensityTierFit(c.tutors, c.totalCards, c.tier)
		if got.Fit >= c.maxFit {
			t.Errorf("%s: want Fit < %.2f, got %.4f", c.name, c.maxFit, got.Fit)
		}
		if got.Direction != c.direction {
			t.Errorf("%s: want %q, got %q", c.name, c.direction, got.Direction)
		}
	}
}

// TestBuildTutorDensityTierFit_BandEdgesNearMidFit verifies the
// "tolerated band" semantic — fit at min/max band edges should be
// ~0.55+.
func TestBuildTutorDensityTierFit_BandEdgesNearMidFit(t *testing.T) {
	for tier, expect := range tierTutorTable {
		minTutors := int(expect.min * 100)
		maxTutors := int(expect.max * 100)
		minGot := BuildTutorDensityTierFit(minTutors, 100, tier)
		maxGot := BuildTutorDensityTierFit(maxTutors, 100, tier)
		if minGot.Fit < 0.55 {
			t.Errorf("tier %d min-edge density %.2f (%d/100): want Fit >= 0.55, got %.4f",
				tier, expect.min, minTutors, minGot.Fit)
		}
		if maxGot.Fit < 0.55 {
			t.Errorf("tier %d max-edge density %.2f (%d/100): want Fit >= 0.55, got %.4f",
				tier, expect.max, maxTutors, maxGot.Fit)
		}
		if minGot.Direction != "in_band" {
			t.Errorf("tier %d at min: want in_band, got %q", tier, minGot.Direction)
		}
		if maxGot.Direction != "in_band" {
			t.Errorf("tier %d at max: want in_band, got %q", tier, maxGot.Direction)
		}
	}
}

// TestBuildTutorDensityTierFit_OutOfRangeTierReturnsZeroFit
// verifies the defensive path.
func TestBuildTutorDensityTierFit_OutOfRangeTierReturnsZeroFit(t *testing.T) {
	for _, tier := range []int{0, -1, 6, 99} {
		got := BuildTutorDensityTierFit(5, 99, tier)
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

// TestBuildTutorDensityTierFit_ZeroTotalCardsReturnsZeroFit
// verifies the no-data path.
func TestBuildTutorDensityTierFit_ZeroTotalCardsReturnsZeroFit(t *testing.T) {
	got := BuildTutorDensityTierFit(5, 0, 3)
	if got.Fit != 0 {
		t.Errorf("zero total cards: want Fit=0, got %.4f", got.Fit)
	}
	if !strings.Contains(got.Verdict, "No card-count data") {
		t.Errorf("zero-cards verdict: %q", got.Verdict)
	}
}

// TestBuildTutorDensityTierFit_VerdictShape verifies the Verdict
// string carries TutorCount / TotalCards / density / TierLabel / fit.
func TestBuildTutorDensityTierFit_VerdictShape(t *testing.T) {
	got := BuildTutorDensityTierFit(4, 99, 3)
	if !strings.Contains(got.Verdict, "4 tutors") {
		t.Errorf("verdict missing tutor count: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "99 cards") {
		t.Errorf("verdict missing total cards: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "Upgraded Precon") {
		t.Errorf("verdict missing tier label: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "fit") {
		t.Errorf("verdict missing 'fit' callout: %q", got.Verdict)
	}
}

// TestBuildTutorDensityTierFit_RealCorpusCalibration runs the fit
// against the real 9-deck cEDH B5 corpus at their classifier-
// determined tier (T5). Aligned cEDH decks should produce fits
// ≥ 0.6 with in_band direction.
//
// Skipped when oracle data absent.
func TestBuildTutorDensityTierFit_RealCorpusCalibration(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)

	matches, _ := filepath.Glob("../../data/decks/test/cedh_*_b5_*.txt")
	if len(matches) == 0 {
		t.Skipf("no cEDH B5 test decks found")
	}

	inBandCount := 0
	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		got := BuildTutorDensityTierFit(report.NonLandTutorCount, report.TotalCards, tier.Tier)
		t.Logf("%s: tutors=%d/%d (%.1f%%) tier=T%d fit=%.3f direction=%s",
			base, report.NonLandTutorCount, report.TotalCards,
			got.ActualDensity*100, tier.Tier, got.Fit, got.Direction)
		if got.Direction == "in_band" {
			inBandCount++
		}
	}
	// Most cEDH decks should be in-band. At least 5 of 9 surface
	// with in_band direction (the noisy stormoff / big_stick decks
	// run a bit lean on tutors, so the floor accounts for them).
	if inBandCount < 5 {
		t.Errorf("expected at least 5/9 cEDH B5 decks in-band, got %d", inBandCount)
	}
}
