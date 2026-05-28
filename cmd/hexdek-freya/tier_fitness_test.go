package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualTF(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestBuildTierFitnessScore_BothComponentsAlignedHighFit verifies
// the canonical happy path: both components in-band + high fit →
// composite Score high, Band="well_tuned", summary mentions
// "well-tuned" + the tier label.
func TestBuildTierFitnessScore_BothComponentsAlignedHighFit(t *testing.T) {
	// T3 Upgraded Precon with ideal CMC + ideal tutor density.
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	if got == nil {
		t.Fatal("nil result")
	}
	if got.Score < 0.85 {
		t.Errorf("aligned T3 deck: want Score >= 0.85, got %.4f", got.Score)
	}
	if got.Band != "well_tuned" {
		t.Errorf("aligned T3 deck: want Band=well_tuned, got %q", got.Band)
	}
	if !strings.Contains(got.Summary, "well-tuned") {
		t.Errorf("summary missing 'well-tuned': %q", got.Summary)
	}
	if !strings.Contains(got.Summary, "Upgraded Precon") {
		t.Errorf("summary missing tier label: %q", got.Summary)
	}
	if got.Tier != 3 {
		t.Errorf("Tier propagation: want 3, got %d", got.Tier)
	}
}

// TestBuildTierFitnessScore_SingleMismatchSurfacesAxis verifies
// the "one axis off, one aligned" case: composite Score lands in
// the mostly_tuned band, summary calls out the weak axis.
func TestBuildTierFitnessScore_SingleMismatchSurfacesAxis(t *testing.T) {
	// T3 declared, curve at 3.2 (ideal — fit 1.0) but tutors only 1
	// (0.5 sigma below T3 min of 3, fit ~0.32). Composite avg ~0.66.
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(1, 99, 3)
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	if got.Band == "well_tuned" {
		t.Errorf("single-mismatch deck shouldn't be well_tuned, got Band=%s Score=%.3f",
			got.Band, got.Score)
	}
	if !strings.Contains(got.Summary, "tutor density too sparse") {
		t.Errorf("summary missing tutor note: %q", got.Summary)
	}
	if strings.Contains(got.Summary, "curve") && !strings.Contains(got.Summary, "tutor") {
		t.Errorf("summary should call out tutors (the weak axis), not curve: %q", got.Summary)
	}
}

// TestBuildTierFitnessScore_BothComponentsDriftedPoorlyTuned
// verifies the "fully off-tier" case: both axes well outside the
// band → low composite Score + poorly_tuned band + summary calls
// out both axes.
func TestBuildTierFitnessScore_BothComponentsDriftedPoorlyTuned(t *testing.T) {
	// T5 declared but curve=3.6 (B2 curve) and tutors=2 (B1-typical)
	curve := BuildManaCurveTierFit(3.6, 5)
	tutors := BuildTutorDensityTierFit(2, 99, 5)
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	if got.Score >= 0.5 {
		t.Errorf("dual-drift cEDH-claim: want Score < 0.5, got %.4f", got.Score)
	}
	if got.Band != "poorly_tuned" {
		t.Errorf("dual-drift: want Band=poorly_tuned, got %q", got.Band)
	}
	if !strings.Contains(got.Summary, "curve") || !strings.Contains(got.Summary, "tutor") {
		t.Errorf("summary should call out BOTH axes on dual-drift: %q", got.Summary)
	}
}

// TestBuildTierFitnessScore_ComponentsSortedByFitAsc verifies the
// weakest-first ordering in Components — consumers iterating for
// "what's wrong?" hit the strongest signal at Components[0].
func TestBuildTierFitnessScore_ComponentsSortedByFitAsc(t *testing.T) {
	// Curve perfect, tutors very sparse → tutors should be first.
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(0, 99, 3)
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	if len(got.Components) != 2 {
		t.Fatalf("want 2 components, got %d", len(got.Components))
	}
	if got.Components[0].Axis != "Tutor Density" {
		t.Errorf("weakest-first sort: want Tutor Density at [0], got %q", got.Components[0].Axis)
	}
	if got.Components[1].Axis != "Mana Curve" {
		t.Errorf("strongest at [1]: want Mana Curve, got %q", got.Components[1].Axis)
	}
}

// TestBuildTierFitnessScore_NilComponentsHandled verifies the
// defensive paths: nil curve, nil tutors, both nil.
func TestBuildTierFitnessScore_NilComponentsHandled(t *testing.T) {
	// Both nil
	got := BuildTierFitnessScore(nil, nil, nil, nil, nil)
	if got == nil {
		t.Fatal("want non-nil result")
	}
	if got.Score != 0 {
		t.Errorf("both nil: want Score=0, got %.4f", got.Score)
	}
	if !strings.Contains(got.Summary, "No tier-fit components") {
		t.Errorf("both nil summary: %q", got.Summary)
	}

	// Only curve
	curve := BuildManaCurveTierFit(3.2, 3)
	got = BuildTierFitnessScore(curve, nil, nil, nil, nil)
	if got.Score < 0.99 {
		t.Errorf("curve-only at ideal: want Score >= 0.99, got %.4f", got.Score)
	}
	if len(got.Components) != 1 {
		t.Errorf("curve-only: want 1 component, got %d", len(got.Components))
	}
	if got.Tier != 3 {
		t.Errorf("Tier propagation from curve: want 3, got %d", got.Tier)
	}

	// Only tutors
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	got = BuildTierFitnessScore(nil, tutors, nil, nil, nil)
	if got.Score < 0.99 {
		t.Errorf("tutors-only at ideal: want Score >= 0.99, got %.4f", got.Score)
	}
	if got.Tier != 3 {
		t.Errorf("Tier propagation from tutors: want 3, got %d", got.Tier)
	}
}

// TestBuildTierFitnessScore_WeightedAverageMath pins the
// arithmetic: with equal weights, composite = mean of component
// fits.
func TestBuildTierFitnessScore_WeightedAverageMath(t *testing.T) {
	curve := BuildManaCurveTierFit(3.2, 3)   // fit ~1.0
	tutors := BuildTutorDensityTierFit(0, 99, 3) // fit ~0.6
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	wantMean := (curve.Fit + tutors.Fit) / 2
	if !almostEqualTF(got.Score, wantMean) {
		t.Errorf("composite score: want %.6f (mean of components), got %.6f",
			wantMean, got.Score)
	}
}

// TestTierFitnessBandFor_Thresholds pins the band thresholds at
// every edge.
func TestTierFitnessBandFor_Thresholds(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{1.00, "well_tuned"},
		{0.85, "well_tuned"},
		{0.849, "mostly_tuned"},
		{0.70, "mostly_tuned"},
		{0.699, "mixed_tuning"},
		{0.50, "mixed_tuning"},
		{0.499, "poorly_tuned"},
		{0.00, "poorly_tuned"},
	}
	for _, c := range cases {
		got := tierFitnessBandFor(c.score)
		if got != c.want {
			t.Errorf("score=%.3f: want %q, got %q", c.score, c.want, got)
		}
	}
}

// TestBuildTierFitnessScore_SummaryFormat verifies the summary
// string carries the expected fields: tier number, tier label,
// percentage, and conditional weak-axis notes.
func TestBuildTierFitnessScore_SummaryFormat(t *testing.T) {
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	if !strings.Contains(got.Summary, "T3") {
		t.Errorf("summary missing tier number: %q", got.Summary)
	}
	if !strings.Contains(got.Summary, "%") {
		t.Errorf("summary missing percentage: %q", got.Summary)
	}
	if !strings.HasSuffix(got.Summary, ".") {
		t.Errorf("summary missing trailing period: %q", got.Summary)
	}
}

// TestBuildTierFitnessScore_WeakNotesCappedAtTwo verifies the
// two-note cap on summary — even with multiple weak axes the
// summary stays single-line.
func TestBuildTierFitnessScore_WeakNotesCappedAtTwo(t *testing.T) {
	// Both axes drifted — should still produce a clean summary
	// with both notes (≤2 cap).
	curve := BuildManaCurveTierFit(4.5, 5)  // way too expensive for T5
	tutors := BuildTutorDensityTierFit(0, 99, 5) // way too sparse
	got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
	// Count " and " (the join separator) — should appear ≤1 time
	// (joining at most 2 notes).
	if strings.Count(got.Summary, " and ") > 1 {
		t.Errorf("summary has too many 'and' joins (>2 notes): %q", got.Summary)
	}
}

// TestBuildTierFitnessScore_RealCorpusCalibration runs the
// composite against the 9-deck cEDH B5 corpus. Aligned cEDH decks
// should produce Score >= 0.70 at T5 (real builds may dip into
// mostly_tuned but shouldn't fall below mixed_tuning).
//
// Skipped when oracle data absent.
func TestBuildTierFitnessScore_RealCorpusCalibration(t *testing.T) {
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

	goodFitCount := 0
	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)

		curve := BuildManaCurveTierFit(dp.AvgCMC, tier.Tier)
		tutors := BuildTutorDensityTierFit(
			report.NonLandTutorCount, report.TotalCards, tier.Tier)
		got := BuildTierFitnessScore(curve, tutors, nil, nil, nil)
		t.Logf("%s: T%d score=%.3f band=%s — %s",
			base, tier.Tier, got.Score, got.Band, got.Summary)
		if got.Score >= 0.70 {
			goodFitCount++
		}
	}
	// At least 6 of 9 cEDH decks should land mostly_tuned+ at T5.
	if goodFitCount < 6 {
		t.Errorf("expected at least 6/9 cEDH B5 decks at Score >= 0.70, got %d",
			goodFitCount)
	}
}
