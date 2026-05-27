package main

import (
	"strings"
	"testing"
)

// makeCurveFitReport returns the minimal FreyaReport scaffolding
// computeCurveArchetypeFit needs: a NonlandCount >= 20 (so the
// short-deck guard doesn't fire).
func makeCurveFitReport() *FreyaReport {
	return &FreyaReport{NonlandCount: 65}
}

// TestComputeCurveArchetypeFit_ComboTooHeavy pins the canonical
// mismatch warning: a deck classified as Combo with a heavy curve
// (AvgCMC well above the 2.7 max) gets flagged "curve is too heavy
// for the archetype's tempo".
func TestComputeCurveArchetypeFit_ComboTooHeavy(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Combo", AvgCMC: 3.8}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(dp.CurveArchetypeWarnings), dp.CurveArchetypeWarnings)
	}
	got := dp.CurveArchetypeWarnings[0]
	for _, want := range []string{"Combo", "lean", "3.8", "too heavy"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q missing %q", got, want)
		}
	}
}

// TestComputeCurveArchetypeFit_ControlTooLight pins the inverse: a
// Control deck with avgCMC well below the 3.0 minimum is flagged
// "curve plays faster than the archetype gameplan needs". Catches the
// "I built Control but ran out of late-game answers" failure mode.
func TestComputeCurveArchetypeFit_ControlTooLight(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Control", AvgCMC: 2.4}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(dp.CurveArchetypeWarnings), dp.CurveArchetypeWarnings)
	}
	got := dp.CurveArchetypeWarnings[0]
	for _, want := range []string{"Control", "top-heavy", "2.4", "faster than"} {
		if !strings.Contains(got, want) {
			t.Errorf("warning %q missing %q", got, want)
		}
	}
}

// TestComputeCurveArchetypeFit_InBandNoWarning pins that an avgCMC
// inside the expected band produces NO warning — a Combo deck at
// avgCMC 2.4 is exactly where Combo wants to live.
func TestComputeCurveArchetypeFit_InBandNoWarning(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Combo", AvgCMC: 2.4}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 0 {
		t.Errorf("in-band curve produced unwanted warnings: %v", dp.CurveArchetypeWarnings)
	}
}

// TestComputeCurveArchetypeFit_ToleranceSlack pins the ±0.2 tolerance.
// Combo's max is 2.7; an avgCMC of 2.85 is within tolerance and must
// NOT trip a warning (most Combo decks pack at least one big finisher
// that nudges the average past the strict max).
func TestComputeCurveArchetypeFit_ToleranceSlack(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Combo", AvgCMC: 2.85}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 0 {
		t.Errorf("AvgCMC 2.85 within tolerance of Combo max 2.7+0.2, should not warn: %v", dp.CurveArchetypeWarnings)
	}
	// Pushing past the tolerance (Combo max + 0.21) DOES warn.
	dp2 := &DeckProfile{PrimaryArchetype: "Combo", AvgCMC: 2.91}
	computeCurveArchetypeFit(dp2, makeCurveFitReport())
	if len(dp2.CurveArchetypeWarnings) != 1 {
		t.Errorf("AvgCMC 2.91 exceeds tolerance of Combo max 2.7+0.2, should warn: got %v", dp2.CurveArchetypeWarnings)
	}
}

// TestComputeCurveArchetypeFit_MidrangeSkipped pins that Midrange has
// no expectation entry and therefore produces no archetype-curve
// warning regardless of AvgCMC — Midrange is the catch-all archetype
// and shape signals are owned by the report.CurveWarnings pass.
func TestComputeCurveArchetypeFit_MidrangeSkipped(t *testing.T) {
	for _, avg := range []float64{1.5, 3.0, 5.0} {
		dp := &DeckProfile{PrimaryArchetype: "Midrange", AvgCMC: avg}
		computeCurveArchetypeFit(dp, makeCurveFitReport())
		if len(dp.CurveArchetypeWarnings) != 0 {
			t.Errorf("Midrange should never warn (avg %.1f), got %v", avg, dp.CurveArchetypeWarnings)
		}
	}
}

// TestComputeCurveArchetypeFit_EmptyArchetypeNoWarning pins that an
// unclassified deck (PrimaryArchetype == "") produces no warning
// rather than crashing on the map lookup.
func TestComputeCurveArchetypeFit_EmptyArchetypeNoWarning(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "", AvgCMC: 4.0}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 0 {
		t.Errorf("empty archetype should not warn: %v", dp.CurveArchetypeWarnings)
	}
}

// TestComputeCurveArchetypeFit_ShortDeckSkipped pins that decks with
// fewer than 20 nonland cards (test fixtures, partial parses) don't
// trip the warning — AvgCMC is unstable at small sample sizes.
func TestComputeCurveArchetypeFit_ShortDeckSkipped(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Combo", AvgCMC: 5.0}
	shortReport := &FreyaReport{NonlandCount: 8}
	computeCurveArchetypeFit(dp, shortReport)
	if len(dp.CurveArchetypeWarnings) != 0 {
		t.Errorf("short deck (NonlandCount=8) should not warn: %v", dp.CurveArchetypeWarnings)
	}
}

// TestComputeCurveArchetypeFit_TopHeavyArchetypesAcceptHighCMC pins
// that the top-heavy band (Reanimator / Control / Lands Matter /
// Pillowfort / Group Hug / Extra Combats / Artifacts / Superfriends)
// accepts AvgCMC up to its max+tolerance. Reanimator at 4.1 is fine;
// 4.5 trips the warning.
func TestComputeCurveArchetypeFit_TopHeavyArchetypesAcceptHighCMC(t *testing.T) {
	dp := &DeckProfile{PrimaryArchetype: "Reanimator", AvgCMC: 4.1}
	computeCurveArchetypeFit(dp, makeCurveFitReport())
	if len(dp.CurveArchetypeWarnings) != 0 {
		t.Errorf("Reanimator AvgCMC 4.1 fits top-heavy band (2.8-4.2), unexpected warning: %v", dp.CurveArchetypeWarnings)
	}
	dp2 := &DeckProfile{PrimaryArchetype: "Reanimator", AvgCMC: 4.5}
	computeCurveArchetypeFit(dp2, makeCurveFitReport())
	if len(dp2.CurveArchetypeWarnings) != 1 {
		t.Errorf("Reanimator AvgCMC 4.5 exceeds tolerance, should warn: %v", dp2.CurveArchetypeWarnings)
	}
}

// TestComputeCurveArchetypeFit_AllExpectationsHaveValidBands is a
// structural sanity check on the archetypeCurveExpectation map: every
// entry must have Min <= Max and a non-empty Band label, so any future
// addition gets caught by CI rather than silently emitting malformed
// warning text.
func TestComputeCurveArchetypeFit_AllExpectationsHaveValidBands(t *testing.T) {
	if len(archetypeCurveExpectation) == 0 {
		t.Fatal("archetypeCurveExpectation is empty")
	}
	for name, exp := range archetypeCurveExpectation {
		if exp.MinAvgCMC > exp.MaxAvgCMC {
			t.Errorf("%s: Min %.1f > Max %.1f", name, exp.MinAvgCMC, exp.MaxAvgCMC)
		}
		if exp.Band == "" {
			t.Errorf("%s: empty Band label", name)
		}
	}
}
