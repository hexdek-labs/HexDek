package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func almostEqualMBTF(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// TestBuildManaBaseTierFit_IdealGradeGivesFitOne pins Gaussian
// centerpoint behavior — when ActualGrade == IdealGrade, Fit=1.0.
func TestBuildManaBaseTierFit_IdealGradeGivesFitOne(t *testing.T) {
	for tier, expect := range tierManaBaseTable {
		grade := scoreGradeMap[expect.idealScore]
		got := BuildManaBaseTierFit(grade, tier)
		if !almostEqualMBTF(got.Fit, 1.0) {
			t.Errorf("tier %d at ideal grade %s: want Fit=1.0, got %.4f",
				tier, grade, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("tier %d at ideal: want in_band, got %q", tier, got.Direction)
		}
	}
}

// TestBuildManaBaseTierFit_AlignedDecksHighFit verifies spec
// scenarios: cEDH expects A; B1 expects D-F; B3 expects C-B —
// each within band should produce Fit ≥ 0.85.
func TestBuildManaBaseTierFit_AlignedDecksHighFit(t *testing.T) {
	cases := []struct {
		name  string
		tier  int
		grade string
	}{
		{"cEDH A-grade", 5, "A"},
		{"High Power B-grade", 4, "B"},
		{"High Power A-grade", 4, "A"},
		{"Upgraded Precon C-grade", 3, "C"},
		{"Upgraded Precon B-grade", 3, "B"},
		{"Casual T2 D-grade", 2, "D"},
		{"Casual T1 D-grade", 1, "D"},
	}
	for _, c := range cases {
		got := BuildManaBaseTierFit(c.grade, c.tier)
		if got.Fit < 0.85 {
			t.Errorf("%s: want Fit >= 0.85, got %.4f", c.name, got.Fit)
		}
		if got.Direction != "in_band" {
			t.Errorf("%s: want in_band, got %q", c.name, got.Direction)
		}
	}
}

// TestBuildManaBaseTierFit_UnderTunedCEDH verifies the flagship
// under-tuned scenario: cEDH-claimed deck with D-grade mana
// surfaces low fit + too_weak + verdict mentions "under-tuned".
func TestBuildManaBaseTierFit_UnderTunedCEDH(t *testing.T) {
	got := BuildManaBaseTierFit("D", 5)
	if got.Fit >= 0.4 {
		t.Errorf("D-grade cEDH-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_weak" {
		t.Errorf("direction: want too_weak, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "under-tuned") {
		t.Errorf("verdict missing 'under-tuned': %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "turn-3 race") {
		t.Errorf("verdict missing semantic note: %q", got.Verdict)
	}
}

// TestBuildManaBaseTierFit_OverBuiltCasual verifies the pubstomp
// scenario: T1-claimed deck with A-grade mana surfaces too_strong
// + pubstomp verdict.
func TestBuildManaBaseTierFit_OverBuiltCasual(t *testing.T) {
	got := BuildManaBaseTierFit("A", 1)
	if got.Fit >= 0.4 {
		t.Errorf("A-grade T1-claim: want Fit < 0.4, got %.4f", got.Fit)
	}
	if got.Direction != "too_strong" {
		t.Errorf("direction: want too_strong, got %q", got.Direction)
	}
	if !strings.Contains(got.Verdict, "pubstomp") {
		t.Errorf("verdict missing 'pubstomp': %q", got.Verdict)
	}
}

// TestBuildManaBaseTierFit_GradeScoreMapping pins the A=5..F=1
// mapping.
func TestBuildManaBaseTierFit_GradeScoreMapping(t *testing.T) {
	cases := map[string]int{"A": 5, "B": 4, "C": 3, "D": 2, "F": 1}
	for grade, wantScore := range cases {
		got := BuildManaBaseTierFit(grade, 3) // T3 to get a stable comparison
		if got.ActualGradeScore != wantScore {
			t.Errorf("grade %s: want ActualGradeScore=%d, got %d",
				grade, wantScore, got.ActualGradeScore)
		}
	}
}

// TestBuildManaBaseTierFit_BandEdgesNearMidFit verifies fit at
// min/max grade edges lands >= 0.55.
func TestBuildManaBaseTierFit_BandEdgesNearMidFit(t *testing.T) {
	for tier, expect := range tierManaBaseTable {
		minGrade := scoreGradeMap[expect.minScore]
		maxGrade := scoreGradeMap[expect.maxScore]
		minGot := BuildManaBaseTierFit(minGrade, tier)
		maxGot := BuildManaBaseTierFit(maxGrade, tier)
		if minGot.Fit < 0.55 {
			t.Errorf("tier %d min-edge %s: want Fit >= 0.55, got %.4f",
				tier, minGrade, minGot.Fit)
		}
		if maxGot.Fit < 0.55 {
			t.Errorf("tier %d max-edge %s: want Fit >= 0.55, got %.4f",
				tier, maxGrade, maxGot.Fit)
		}
		if minGot.Direction != "in_band" || maxGot.Direction != "in_band" {
			t.Errorf("tier %d edges: want in_band, got min=%q max=%q",
				tier, minGot.Direction, maxGot.Direction)
		}
	}
}

// TestBuildManaBaseTierFit_DefensivePaths verifies out-of-range
// tier + empty/unknown grade.
func TestBuildManaBaseTierFit_DefensivePaths(t *testing.T) {
	// Out-of-range tier
	for _, tier := range []int{0, -1, 6, 99} {
		got := BuildManaBaseTierFit("A", tier)
		if got.Fit != 0 {
			t.Errorf("tier %d: want Fit=0, got %.4f", tier, got.Fit)
		}
		if got.TierLabel != "Unknown" {
			t.Errorf("tier %d: want TierLabel=Unknown, got %q", tier, got.TierLabel)
		}
	}
	// Empty grade
	got := BuildManaBaseTierFit("", 5)
	if got.Fit != 0 {
		t.Errorf("empty grade: want Fit=0, got %.4f", got.Fit)
	}
	if !strings.Contains(got.Verdict, "No mana-base grade data") {
		t.Errorf("empty grade verdict: %q", got.Verdict)
	}
	// Unknown grade
	got = BuildManaBaseTierFit("X", 5)
	if got.Fit != 0 {
		t.Errorf("unknown grade: want Fit=0, got %.4f", got.Fit)
	}
}

// TestBuildManaBaseTierFit_VerdictShape verifies key fields appear.
func TestBuildManaBaseTierFit_VerdictShape(t *testing.T) {
	got := BuildManaBaseTierFit("C", 3)
	if !strings.Contains(got.Verdict, "grade C") {
		t.Errorf("verdict missing actual grade: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "Upgraded Precon") {
		t.Errorf("verdict missing tier label: %q", got.Verdict)
	}
	if !strings.Contains(got.Verdict, "fit") {
		t.Errorf("verdict missing 'fit': %q", got.Verdict)
	}
}

// TestBuildManaBaseTierFit_TierFitnessAggregateIntegration verifies
// the new component plugs into BuildTierFitnessScore correctly.
func TestBuildManaBaseTierFit_TierFitnessAggregateIntegration(t *testing.T) {
	// T3 declared, all 5 components ideal → composite well_tuned.
	curve := BuildManaCurveTierFit(3.2, 3)
	tutors := BuildTutorDensityTierFit(4, 99, 3)
	interaction := BuildInteractionTierFit(mkInteractionReport(11), 3)
	removal := BuildRemovalTierFit(mkRemovalReport(8, 0), 3)
	manaBase := BuildManaBaseTierFit("C", 3)
	got := BuildTierFitnessScore(curve, tutors, interaction, removal, manaBase)
	if len(got.Components) != 5 {
		t.Errorf("want 5 components, got %d", len(got.Components))
	}
	if got.Score < 0.99 {
		t.Errorf("all-ideal composite: want Score >= 0.99, got %.4f", got.Score)
	}

	// Extreme mana-base drift (T5 ideal A vs F = 4 grades off, fit
	// 0.135) on a T5 deck → composite Score drops out of
	// well_tuned band even with 4 perfect components, and summary
	// calls out the mana base.
	curve5 := BuildManaCurveTierFit(2.0, 5)
	tutors5 := BuildTutorDensityTierFit(12, 99, 5)
	interaction5 := BuildInteractionTierFit(mkInteractionReport(17), 5)
	removal5 := BuildRemovalTierFit(mkRemovalReport(8, 0), 5)
	manaBase = BuildManaBaseTierFit("F", 5)
	got = BuildTierFitnessScore(curve5, tutors5, interaction5, removal5, manaBase)
	if got.Band == "well_tuned" {
		t.Errorf("F-grade T5 mana-base-drift: want Band != well_tuned, got %s (Score=%.3f)",
			got.Band, got.Score)
	}
	if !strings.Contains(got.Summary, "mana base") {
		t.Errorf("summary should mention mana base drift: %q", got.Summary)
	}
}

// TestBuildManaBaseTierFit_RealCorpusCalibration runs the fit
// against the 9-deck cEDH B5 corpus.
func TestBuildManaBaseTierFit_RealCorpusCalibration(t *testing.T) {
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
		got := BuildManaBaseTierFit(dp.ManaBaseGrade, tier.Tier)
		t.Logf("%s: grade=%s tier=T%d fit=%.3f direction=%s",
			base, dp.ManaBaseGrade, tier.Tier, got.Fit, got.Direction)
		if got.Direction == "in_band" {
			inBandCount++
		}
	}
	if inBandCount < 5 {
		t.Errorf("expected at least 5/9 cEDH B5 decks in-band on mana base, got %d",
			inBandCount)
	}
}
