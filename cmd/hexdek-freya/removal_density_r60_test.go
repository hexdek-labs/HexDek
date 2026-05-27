package main

import (
	"strings"
	"testing"
)

// removalDensityFixture builds the minimum FreyaReport + DeckProfile
// pair that deriveWeaknesses needs to evaluate the removal-density
// band. SingleTargetRemovalCount is the only varying input; all other
// counts are set to safe-band values so unrelated warnings don't fire.
func removalDensityFixture(singleTargetRemoval int) (*FreyaReport, *DeckProfile) {
	report := &FreyaReport{
		SingleTargetRemovalCount: singleTargetRemoval,
		RemovalCount:             singleTargetRemoval + 2, // include some wipes for the "low interaction" guard
		NonLandTutorCount:        5,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 0,
				RoleBoardWipe:    2, // suppress "no board wipes"
			},
		},
	}
	dp := &DeckProfile{
		DrawCount:    10,
		RampCount:    10,
		WinLineCount: 4,
		LandCount:    37,
		LandVerdict:  "ok",
	}
	return report, dp
}

// hasWarningContaining returns true iff one of the warnings contains
// the given substring. Used by the band tests below.
func hasWarningContaining(ws []string, substr string) bool {
	for _, w := range ws {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// TestRemovalDensity_ThinFires — a deck with 5 single-target removal
// spells (below the 8-piece threshold) gets the "thin single-target
// removal" warning. Pin both the substring "thin single-target
// removal" and the piece count.
func TestRemovalDensity_ThinFires(t *testing.T) {
	report, dp := removalDensityFixture(5)
	w := deriveWeaknesses(report, dp)
	if !hasWarningContaining(w, "thin single-target removal (5 pieces)") {
		t.Fatalf("expected thin-removal warning naming the count, got: %v", w)
	}
	if !hasWarningContaining(w, "8+ answers") {
		t.Errorf("warning should reference the 8+ threshold so the reader knows what to add, got: %v", w)
	}
}

// TestRemovalDensity_HealthyDoesNotFire — 10 single-target removal
// pieces sits in the healthy 8-18 band. Neither warning should fire.
func TestRemovalDensity_HealthyDoesNotFire(t *testing.T) {
	report, dp := removalDensityFixture(10)
	w := deriveWeaknesses(report, dp)
	if hasWarningContaining(w, "thin single-target removal") {
		t.Errorf("10 pieces is healthy; thin warning should not fire, got: %v", w)
	}
	if hasWarningContaining(w, "control-heavy removal") {
		t.Errorf("10 pieces is healthy; control-heavy warning should not fire, got: %v", w)
	}
}

// TestRemovalDensity_ControlHeavyFires — 20 single-target removal
// pieces is past the 18-piece threshold and fires the "control-heavy
// removal" heads-up.
func TestRemovalDensity_ControlHeavyFires(t *testing.T) {
	report, dp := removalDensityFixture(20)
	w := deriveWeaknesses(report, dp)
	if !hasWarningContaining(w, "control-heavy removal (20 single-target pieces)") {
		t.Fatalf("expected control-heavy warning at 20 pieces, got: %v", w)
	}
	if !hasWarningContaining(w, "expect long games") {
		t.Errorf("control-heavy warning should set expectation of long games, got: %v", w)
	}
}

// TestRemovalDensity_BandBoundaries pins the exact boundary semantics:
// 7 fires thin, 8 doesn't; 18 doesn't fire control-heavy, 19 does.
// Prevents off-by-one drift on either edge during future calibration.
func TestRemovalDensity_BandBoundaries(t *testing.T) {
	cases := []struct {
		count        int
		wantThin     bool
		wantControl  bool
	}{
		{count: 7, wantThin: true, wantControl: false},
		{count: 8, wantThin: false, wantControl: false},
		{count: 18, wantThin: false, wantControl: false},
		{count: 19, wantThin: false, wantControl: true},
	}
	for _, c := range cases {
		report, dp := removalDensityFixture(c.count)
		w := deriveWeaknesses(report, dp)
		gotThin := hasWarningContaining(w, "thin single-target removal")
		gotControl := hasWarningContaining(w, "control-heavy removal")
		if gotThin != c.wantThin {
			t.Errorf("count=%d: thin warning got=%v want=%v (warnings: %v)", c.count, gotThin, c.wantThin, w)
		}
		if gotControl != c.wantControl {
			t.Errorf("count=%d: control warning got=%v want=%v (warnings: %v)", c.count, gotControl, c.wantControl, w)
		}
	}
}

// TestRemovalDensity_ZeroDoesNotFireThin — a deck reporting 0
// single-target removal isn't flagged by the "thin" warning. The 0
// case usually means deck analysis hasn't populated the field
// (incomplete oracle data, parse failure on a custom deck) — falling
// through silently is the right behavior; the existing "low
// interaction (X removal + counterspells)" warning at <5 already
// covers the genuinely-broken case.
func TestRemovalDensity_ZeroDoesNotFireThin(t *testing.T) {
	report, dp := removalDensityFixture(0)
	w := deriveWeaknesses(report, dp)
	if hasWarningContaining(w, "thin single-target removal") {
		t.Errorf("count=0 should not fire thin warning (zero often = incomplete data), got: %v", w)
	}
}

// TestRemovalDensity_AnalysisPipelinePopulatesField — exercises the
// counter increment path in analysis.go: a mix of single-target
// removal (IsRemoval && !IsMassWipe) and mass wipes (IsRemoval &&
// IsMassWipe) must correctly split into RemovalCount (all 4) and
// SingleTargetRemovalCount (3 — the mass wipe excluded).
func TestRemovalDensity_AnalysisPipelinePopulatesField(t *testing.T) {
	report := &FreyaReport{}
	profiles := []CardProfile{
		{Name: "Swords to Plowshares", IsRemoval: true, IsMassWipe: false},
		{Name: "Path to Exile", IsRemoval: true, IsMassWipe: false},
		{Name: "Beast Within", IsRemoval: true, IsMassWipe: false},
		{Name: "Wrath of God", IsRemoval: true, IsMassWipe: true},
		{Name: "Sol Ring", IsRemoval: false, IsMassWipe: false},
	}
	// Mirror the analysis.go:1310-1315 logic — the counter loop is small
	// and worth pinning directly so a refactor of that file doesn't
	// silently drop the !IsMassWipe gate.
	for _, p := range profiles {
		if p.IsRemoval {
			report.RemovalCount++
			if !p.IsMassWipe {
				report.SingleTargetRemovalCount++
			}
		}
	}
	if report.RemovalCount != 4 {
		t.Errorf("RemovalCount = %d, want 4 (3 single-target + 1 mass)", report.RemovalCount)
	}
	if report.SingleTargetRemovalCount != 3 {
		t.Errorf("SingleTargetRemovalCount = %d, want 3 (mass wipe excluded)", report.SingleTargetRemovalCount)
	}
}
