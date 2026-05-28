package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualITF(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// mkInteractionReport synthesizes a FreyaReport with the role
// counts BuildInteractionTierFit consumes. The interaction count
// equals Removal + BoardWipe + Counterspell + Protection so the
// test rolls them into one configurable arg.
func mkInteractionReport(interactionPieces int) *FreyaReport {
	return &FreyaReport{
		Roles: &RoleAnalysis{
			TotalCards: 99,
			RoleCounts: map[RoleTag]int{
				RoleRemoval: interactionPieces,
			},
		},
	}
}

// TestBuildInteractionTierFit_IdealCountGivesFitOne pins the
// Gaussian centerpoint: when InteractionCount == IdealCount,
// Fit ≈ 1.0.
func TestBuildInteractionTierFit_IdealCountGivesFitOne(t *testing.T) {
	for tier, expect := range tierInteractionTable {
		report := mkInteractionReport(expect.ideal)
		got := BuildInteractionTierFit(report, tier)
		if !almostEqualITF(got.Fit, 1.0) {
			t.Errorf("tier %d at ideal count %d: want Fit=1.0, got %.4f",
				tier, expect.ideal, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("tier %d at ideal: want in_band, got %q", tier, got.Direction)
		}
	}
}

// TestBuildInteractionTierFit_AlignedDecksHighFit verifies the
// user's spec scenarios: cEDH ≥15, B1 5-8, B3 10-12 — each within
// band should produce Fit ≥ 0.85.
func TestBuildInteractionTierFit_AlignedDecksHighFit(t *testing.T) {
	cases := []struct {
		name string
		tier int
		ix   int
	}{
		{"cEDH at 17", 5, 17},
		{"cEDH at 19", 5, 19},
		{"High Power at 13", 4, 13},
		{"Upgraded Precon at 11", 3, 11},
		{"Upgraded Precon at 10", 3, 10},
		{"Casual T2 at 8", 2, 8},
		{"Casual T1 at 6", 1, 6},
	}
	for _, c := range cases {
		report := mkInteractionReport(c.ix)
		got := BuildInteractionTierFit(report, c.tier)
		if got.Fit < 0.85 {
			t.Errorf("%s: want Fit >= 0.85, got %.4f", c.name, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("%s: want in_band, got %q", c.name, got.Direction)
		}
	}
}

// TestBuildInteractionTierFit_UnderInteractedCEDH verifies the
// flagship spec scenario: a cEDH-claimed deck with only 6
// interaction pieces (T1-casual level) surfaces low fit +
// too_sparse + verdict mentions "under-tuned" / "can't survive".
func TestBuildInteractionTierFit_UnderInteractedCEDH(t *testing.T) {
	report := mkInteractionReport(6)
	got := BuildInteractionTierFit(report, 5)
	if got.Fit >= 0.4 {
		t.Errorf("6-piece cEDH deck: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_sparse" {
		t.Errorf("direction: want too_sparse, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "under-tuned") {
		t.Errorf("verdict missing 'under-tuned': %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "race") {
		t.Errorf("verdict missing 'race': %q", got.Verdict)
	}
}

// TestBuildInteractionTierFit_OverInteractedCasual verifies the
// pubstomp scenario: a T1-claimed deck with 18 interaction pieces
// (cEDH-shape) surfaces too_dense + pubstomp verdict.
func TestBuildInteractionTierFit_OverInteractedCasual(t *testing.T) {
	report := mkInteractionReport(18)
	got := BuildInteractionTierFit(report, 1)
	if got.Fit >= 0.4 {
		t.Errorf("18-piece T1-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_dense" {
		t.Errorf("direction: want too_dense, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "pubstomp") {
		t.Errorf("verdict missing 'pubstomp': %q", got.Verdict)
	}
}

// TestBuildInteractionTierFit_BandEdgesNearMidFit verifies the
// "tolerated band" semantic — fit at min/max band edges should be
// ~0.55+.
func TestBuildInteractionTierFit_BandEdgesNearMidFit(t *testing.T) {
	for tier, expect := range tierInteractionTable {
		minGot := BuildInteractionTierFit(mkInteractionReport(expect.min), tier)
		maxGot := BuildInteractionTierFit(mkInteractionReport(expect.max), tier)
		if minGot.Fit < 0.55 {
			t.Errorf("tier %d min-edge count %d: want Fit >= 0.55, got %.4f",
				tier, expect.min, minGot.Fit)
		}
		if maxGot.Fit < 0.55 {
			t.Errorf("tier %d max-edge count %d: want Fit >= 0.55, got %.4f",
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

// TestBuildInteractionTierFit_OutOfRangeTierReturnsZeroFit
// verifies defensive handling.
func TestBuildInteractionTierFit_OutOfRangeTierReturnsZeroFit(t *testing.T) {
	for _, tier := range []int{0, -1, 6, 99} {
		got := BuildInteractionTierFit(mkInteractionReport(10), tier)
		if got.Fit != 0 {
			t.Errorf("tier %d: want Fit=0, got %.4f", tier, got.Fit)
		}
		if got.TierLabel != "Unknown" {
			t.Errorf("tier %d: want TierLabel=Unknown, got %q", tier, got.TierLabel)
		}
	}
}

// TestBuildInteractionTierFit_NilReportReturnsZeroFit verifies
// the nil-report defensive path.
func TestBuildInteractionTierFit_NilReportReturnsZeroFit(t *testing.T) {
	got := BuildInteractionTierFit(nil, 5)
	if got.Fit != 0 {
		t.Errorf("nil report: want Fit=0, got %.4f", got.Fit)
	}
	if !strings.Contains(got.Verdict, "No role data") {
		t.Errorf("nil report verdict: %q", got.Verdict)
	}
	// Empty Roles
	got = BuildInteractionTierFit(&FreyaReport{}, 5)
	if got.Fit != 0 {
		t.Errorf("empty roles: want Fit=0, got %.4f", got.Fit)
	}
}

// TestBuildInteractionTierFit_SumsAllFourRoles verifies the
// interaction count sums Removal + BoardWipe + Counterspell +
// Protection (not just Removal).
func TestBuildInteractionTierFit_SumsAllFourRoles(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			TotalCards: 99,
			RoleCounts: map[RoleTag]int{
				RoleRemoval:      5,
				RoleBoardWipe:    2,
				RoleCounterspell: 5,
				RoleProtection:   3,
			},
		},
	}
	got := BuildInteractionTierFit(report, 3) // T3 ideal=11
	if got.InteractionCount != 15 {
		t.Errorf("InteractionCount: want 15 (5+2+5+3), got %d", got.InteractionCount)
	}
}

// TestBuildInteractionTierFit_VerdictShape verifies the Verdict
// string carries InteractionCount + TierLabel + ideal + fit.
func TestBuildInteractionTierFit_VerdictShape(t *testing.T) {
	got := BuildInteractionTierFit(mkInteractionReport(11), 3)
	if !strings.Contains(got.Verdict, "11 interaction pieces") {
		t.Errorf("verdict missing count: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "Upgraded Precon") {
		t.Errorf("verdict missing tier label: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "fit") {
		t.Errorf("verdict missing 'fit' callout: %q", got.Verdict)
	}
}

// TestBuildInteractionTierFit_TierFitnessAggregateIntegration
// verifies the new component plugs into TierFitnessScore.
func TestBuildInteractionTierFit_TierFitnessAggregateIntegration(t *testing.T) {
	// T3 declared with ideal CMC + ideal tutors + ideal interaction
	// → all 3 components fit 1.0 → composite well_tuned.
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	interaction := BuildInteractionTierFit(mkInteractionReport(11), 3)
	got := BuildTierFitnessScore(curve, tutors, interaction, nil)
	if len(got.Components) != 3 {
		t.Errorf("want 3 components, got %d", len(got.Components))
	}
	if got.Score < 0.99 {
		t.Errorf("all-ideal composite: want Score >= 0.99, got %.4f", got.Score)
	}

	// Now drift interaction to test it surfaces in the summary
	interaction = BuildInteractionTierFit(mkInteractionReport(3), 3)
	got = BuildTierFitnessScore(curve, tutors, interaction, nil)
	if got.Band == "well_tuned" {
		t.Errorf("interaction-drift deck shouldn't be well_tuned, got %s", got.Band)
	}
	if !strings.Contains(got.Summary, "interaction") {
		t.Errorf("summary should mention interaction drift: %q", got.Summary)
	}
}

// TestBuildInteractionTierFit_RealCorpusCalibration runs the fit
// against the 9-deck cEDH B5 corpus at their classifier-determined
// tier. At least 5/9 should land in_band at T5.
func TestBuildInteractionTierFit_RealCorpusCalibration(t *testing.T) {
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
		got := BuildInteractionTierFit(report, tier.Tier)
		t.Logf("%s: interaction=%d tier=T%d fit=%.3f direction=%s",
			base, got.InteractionCount, tier.Tier, got.Fit, got.Direction)
		if got.Direction == "in_band" {
			inBandCount++
		}
	}
	if inBandCount < 5 {
		t.Errorf("expected at least 5/9 cEDH B5 decks in-band on interaction, got %d",
			inBandCount)
	}
}
