package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualRTF(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// mkRemovalReport synthesizes a FreyaReport with RoleRemoval +
// RoleBoardWipe counts. removal+wipe = total piece count.
func mkRemovalReport(removal, wipe int) *FreyaReport {
	return &FreyaReport{
		Roles: &RoleAnalysis{
			TotalCards: 99,
			RoleCounts: map[RoleTag]int{
				RoleRemoval:   removal,
				RoleBoardWipe: wipe,
			},
		},
	}
}

// TestBuildRemovalTierFit_IdealCountGivesFitOne pins Gaussian
// centerpoint behavior.
func TestBuildRemovalTierFit_IdealCountGivesFitOne(t *testing.T) {
	for tier, expect := range tierRemovalTable {
		report := mkRemovalReport(expect.ideal, 0)
		got := BuildRemovalTierFit(report, tier)
		if !almostEqualRTF(got.Fit, 1.0) {
			t.Errorf("tier %d at ideal count %d: want Fit=1.0, got %.4f",
				tier, expect.ideal, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("tier %d at ideal: want in_band, got %q", tier, got.Direction)
		}
	}
}

// TestBuildRemovalTierFit_AlignedDecksHighFit verifies spec
// scenarios: cEDH 8+, B1 4-6, B3 6-10 — each within band should
// produce Fit ≥ 0.85.
func TestBuildRemovalTierFit_AlignedDecksHighFit(t *testing.T) {
	cases := []struct {
		name    string
		tier    int
		removal int
	}{
		{"cEDH at 8", 5, 8},
		{"cEDH at 7", 5, 7},
		{"cEDH at 9", 5, 9},
		{"High Power at 8", 4, 8},
		{"High Power at 9", 4, 9},
		{"Upgraded Precon at 8", 3, 8},
		{"Upgraded Precon at 7", 3, 7},
		{"Upgraded Precon at 9", 3, 9},
		{"Casual T2 at 6", 2, 6},
		{"Casual T1 at 5", 1, 5},
	}
	for _, c := range cases {
		report := mkRemovalReport(c.removal, 0)
		got := BuildRemovalTierFit(report, c.tier)
		if got.Fit < 0.85 {
			t.Errorf("%s: want Fit >= 0.85, got %.4f", c.name, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("%s: want in_band, got %q", c.name, got.Direction)
		}
	}
}

// TestBuildRemovalTierFit_UnderRemovalCEDH verifies the flagship
// under-tuned scenario: cEDH-claim with 3 removal pieces (well
// below the 6-piece T5 floor) surfaces low fit + too_sparse +
// verdict mentioning under-tuned shape.
func TestBuildRemovalTierFit_UnderRemovalCEDH(t *testing.T) {
	report := mkRemovalReport(3, 0)
	got := BuildRemovalTierFit(report, 5)
	if got.Fit >= 0.4 {
		t.Errorf("3-piece cEDH-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_sparse" {
		t.Errorf("direction: want too_sparse, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "under-tuned") {
		t.Errorf("verdict missing 'under-tuned': %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "answer opponent threats") {
		t.Errorf("verdict missing semantic note: %q", got.Verdict)
	}
}

// TestBuildRemovalTierFit_OverRemovalCasual verifies the pubstomp
// scenario: T1-claim with 15 removal pieces.
func TestBuildRemovalTierFit_OverRemovalCasual(t *testing.T) {
	report := mkRemovalReport(15, 0)
	got := BuildRemovalTierFit(report, 1)
	if got.Fit >= 0.4 {
		t.Errorf("15-piece T1-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_dense" {
		t.Errorf("direction: want too_dense, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "pubstomp") {
		t.Errorf("verdict missing 'pubstomp': %q", got.Verdict)
	}
}

// TestBuildRemovalTierFit_SumsRemovalPlusWipes verifies the
// arithmetic: total = Removal + BoardWipe.
func TestBuildRemovalTierFit_SumsRemovalPlusWipes(t *testing.T) {
	// 6 single-target + 2 wipes = 8 total (T3 ideal)
	report := mkRemovalReport(6, 2)
	got := BuildRemovalTierFit(report, 3)
	if got.RemovalCount != 8 {
		t.Errorf("RemovalCount: want 8 (6+2), got %d", got.RemovalCount)
	}
	if got.Fit < 0.99 {
		t.Errorf("at ideal count via wipes: want Fit >= 0.99, got %.4f", got.Fit)
	}
}

// TestBuildRemovalTierFit_BandEdgesNearMidFit verifies fit at band
// edges lands ≥ 0.55.
func TestBuildRemovalTierFit_BandEdgesNearMidFit(t *testing.T) {
	for tier, expect := range tierRemovalTable {
		minGot := BuildRemovalTierFit(mkRemovalReport(expect.min, 0), tier)
		maxGot := BuildRemovalTierFit(mkRemovalReport(expect.max, 0), tier)
		if minGot.Fit < 0.55 {
			t.Errorf("tier %d min-edge %d: want Fit >= 0.55, got %.4f",
				tier, expect.min, minGot.Fit)
		}
		if maxGot.Fit < 0.55 {
			t.Errorf("tier %d max-edge %d: want Fit >= 0.55, got %.4f",
				tier, expect.max, maxGot.Fit)
		}
		if minGot.Direction != "in_band" || maxGot.Direction != "in_band" {
			t.Errorf("tier %d edges: want in_band, got min=%q max=%q",
				tier, minGot.Direction, maxGot.Direction)
		}
	}
}

// TestBuildRemovalTierFit_DefensivePaths verifies out-of-range
// tier + nil report.
func TestBuildRemovalTierFit_DefensivePaths(t *testing.T) {
	for _, tier := range []int{0, -1, 6, 99} {
		got := BuildRemovalTierFit(mkRemovalReport(5, 0), tier)
		if got.Fit != 0 {
			t.Errorf("tier %d: want Fit=0, got %.4f", tier, got.Fit)
		}
		if got.TierLabel != "Unknown" {
			t.Errorf("tier %d: want TierLabel=Unknown, got %q", tier, got.TierLabel)
		}
	}
	got := BuildRemovalTierFit(nil, 5)
	if got.Fit != 0 {
		t.Errorf("nil report: want Fit=0, got %.4f", got.Fit)
	}
	if !strings.Contains(got.Verdict, "No role data") {
		t.Errorf("nil report verdict: %q", got.Verdict)
	}
}

// TestBuildRemovalTierFit_VerdictShape verifies key fields appear.
func TestBuildRemovalTierFit_VerdictShape(t *testing.T) {
	got := BuildRemovalTierFit(mkRemovalReport(8, 0), 3)
	if !strings.Contains(got.Verdict, "8 removal pieces") {
		t.Errorf("verdict missing count: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "Upgraded Precon") {
		t.Errorf("verdict missing tier label: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "fit") {
		t.Errorf("verdict missing 'fit': %q", got.Verdict)
	}
}

// TestBuildRemovalTierFit_TierFitnessAggregateIntegration verifies
// the new component plugs into BuildTierFitnessScore correctly.
func TestBuildRemovalTierFit_TierFitnessAggregateIntegration(t *testing.T) {
	// T3 declared, all 4 components ideal → composite well_tuned
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	interaction := BuildInteractionTierFit(mkInteractionReport(11), 3)
	removal := BuildRemovalTierFit(mkRemovalReport(8, 0), 3)
	got := BuildTierFitnessScore(curve, tutors, interaction, removal)
	if len(got.Components) != 4 {
		t.Errorf("want 4 components, got %d", len(got.Components))
	}
	if got.Score < 0.99 {
		t.Errorf("all-ideal composite: want Score >= 0.99, got %.4f", got.Score)
	}

	// Drift removal → should surface in summary
	removal = BuildRemovalTierFit(mkRemovalReport(2, 0), 3)
	got = BuildTierFitnessScore(curve, tutors, interaction, removal)
	if got.Band == "well_tuned" {
		t.Errorf("removal-drift deck shouldn't be well_tuned, got %s", got.Band)
	}
	if !strings.Contains(got.Summary, "removal") {
		t.Errorf("summary should mention removal drift: %q", got.Summary)
	}
}

// TestBuildRemovalTierFit_RealCorpusCalibration runs the fit
// against the 9-deck cEDH B5 corpus.
func TestBuildRemovalTierFit_RealCorpusCalibration(t *testing.T) {
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
		got := BuildRemovalTierFit(report, tier.Tier)
		t.Logf("%s: removal=%d tier=T%d fit=%.3f direction=%s",
			base, got.RemovalCount, tier.Tier, got.Fit, got.Direction)
		if got.Direction == "in_band" {
			inBandCount++
		}
	}
	if inBandCount < 4 {
		t.Errorf("expected at least 4/9 cEDH B5 decks in-band on removal, got %d",
			inBandCount)
	}
}
