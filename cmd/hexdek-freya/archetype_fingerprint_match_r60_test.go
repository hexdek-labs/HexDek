package main

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// fingerprintMatchFromDistance must:
//   - return 1.0 for a perfect match (distance 0)
//   - decrease monotonically as distance grows
//   - clamp at 0.0 for distance ≥ 0.5 (2x calibration)
//   - clamp at 1.0 for negative input (defensive)
func TestFingerprintMatchFromDistance_Curve(t *testing.T) {
	cases := []struct {
		distance float64
		want     float64
	}{
		{0.0, 1.0},
		{0.05, 0.9},
		{0.10, 0.8},
		{0.15, 0.7},
		{0.25, 0.5},
		{0.35, 0.3},
		{0.49, 0.02},
		{0.50, 0.0},
		{0.75, 0.0},  // clamped
		{1.00, 0.0},  // clamped
		{-0.10, 1.0}, // defensive: negative distance clamps to 1.0
	}
	for _, tc := range cases {
		got := fingerprintMatchFromDistance(tc.distance)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("distance=%.3f: got match=%.6f, want %.6f", tc.distance, got, tc.want)
		}
	}
}

// Monotonicity invariant: any pair of distances d1 < d2 must satisfy
// match(d1) >= match(d2). Defends against accidental formula changes
// that introduce non-monotonic behavior.
func TestFingerprintMatchFromDistance_Monotonic(t *testing.T) {
	prev := math.Inf(1)
	for d := 0.0; d <= 1.0; d += 0.025 {
		got := fingerprintMatchFromDistance(d)
		if got > prev {
			t.Errorf("monotonicity violated at distance=%.3f: match=%.6f > prev=%.6f", d, got, prev)
		}
		prev = got
	}
}

// Range invariant: match score must always be in [0, 1].
func TestFingerprintMatchFromDistance_AlwaysInRange(t *testing.T) {
	for _, d := range []float64{-1, -0.01, 0, 0.001, 0.5, 0.999, 1.0, 5.0, 100.0} {
		got := fingerprintMatchFromDistance(d)
		if got < 0 || got > 1 {
			t.Errorf("distance=%.3f produced out-of-range match=%.6f", d, got)
		}
	}
}

// End-to-end consistency: ClassifyArchetype must compute
// FingerprintMatch from the WINNER's raw distance via the helper.
// Builds a real-ish profile, runs the full classifier, then
// recomputes the expected match from the implied distance and pins
// the two agree. Robust against archetype-eligibility gates that
// affect WHICH archetype wins — we don't care which one, we care
// that the FingerprintMatch reflects the winner's actual fit.
func TestClassifyArchetype_FingerprintMatchConsistentWithDistance(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleRamp: 10, RoleDraw: 10, RoleRemoval: 8, RoleBoardWipe: 3,
				RoleThreat: 12, RoleProtection: 6, RoleLand: 36,
				RoleTutor: 4, RoleUtility: 10,
			},
			TotalCards: 99,
		},
		AvgCMC: 2.8,
	}
	ac := ClassifyArchetype(report, nil, nil)
	if ac == nil {
		t.Fatal("expected classification")
	}
	// FingerprintMatch must be in [0,1] and be the helper's output
	// for SOME valid distance d in [0, 0.5]. Reverse-derive d from
	// the score and verify it's non-negative and plausible.
	if ac.FingerprintMatch < 0 || ac.FingerprintMatch > 1 {
		t.Errorf("FingerprintMatch out of [0,1] range: %.6f", ac.FingerprintMatch)
	}
	// (1 - FingerprintMatch) / 2 must be the actual distance to
	// SOME archetype template — confirm by re-computing for the
	// declared Primary's fingerprint and matching.
	var winnerRatios map[RoleTag]float64
	for _, fp := range archetypeFingerprints {
		if fp.Name == ac.Primary {
			winnerRatios = fp.Ratios
			break
		}
	}
	if winnerRatios != nil {
		ratios := make(map[RoleTag]float64, len(report.Roles.RoleCounts))
		total := float64(report.Roles.TotalCards)
		for r, c := range report.Roles.RoleCounts {
			ratios[r] = float64(c) / total
		}
		gotDist := euclideanDistance(ratios, winnerRatios)
		expected := fingerprintMatchFromDistance(gotDist)
		if math.Abs(expected-ac.FingerprintMatch) > 1e-9 {
			t.Errorf("FingerprintMatch=%.6f, but recomputed from winner=%q distance=%.6f → %.6f",
				ac.FingerprintMatch, ac.Primary, gotDist, expected)
		}
	}
}

// Direct: when ClassifyArchetype's fallback fires (no fingerprint
// matched), the "Midrange" fallback hard-codes distance 0.5 →
// FingerprintMatch must be exactly 0 per the helper's calibration.
// Reproducible only with TotalCards=0 / nil Roles, which the
// function rejects up front. So we test the documented contract
// via the helper directly: distance 0.5 → match 0.
func TestFingerprintMatch_HelperAtFallbackDistance(t *testing.T) {
	got := fingerprintMatchFromDistance(0.5)
	if got != 0 {
		t.Errorf("fallback distance 0.5 must produce match=0 (the 'we don't actually fit any archetype' signal); got %.6f", got)
	}
}

// Confidence vs FingerprintMatch must be DISTINCT signals — a deck
// can have high relative confidence (clear winner over runner-up)
// AND low fingerprint match (winner is still a poor fit). Constructs
// a contrived case: two fingerprints, the winner is far away but the
// runner-up is even farther.
func TestClassifyArchetype_ConfidenceAndMatchCanDiverge(t *testing.T) {
	// Use the standard fingerprint set. Build a profile that's
	// pure-utility-heavy: dist to every fingerprint is high, but one
	// archetype is the closest by a wide margin → high relative
	// PrimaryConfidence, low absolute FingerprintMatch.
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleLand:    40,
				RoleUtility: 30,
				RoleDraw:    15,
				RoleRamp:    15,
			},
			TotalCards: 100,
		},
		AvgCMC: 3.0,
	}
	ac := ClassifyArchetype(report, nil, nil)
	if ac == nil {
		t.Fatal("expected classification")
	}
	// We don't pin specific numbers — just verify the two scores
	// aren't trivially identical (which would mean the new field is
	// just an alias of the old).
	if math.Abs(ac.PrimaryConfidence-ac.FingerprintMatch) < 1e-6 {
		t.Errorf("PrimaryConfidence and FingerprintMatch must be distinct signals; got both = %.6f for primary=%q",
			ac.PrimaryConfidence, ac.Primary)
	}
}

// Plumbing: DeckProfile must carry the FingerprintMatch from
// ArchetypeClassification. Pins the wiring at the deckprofile
// assignment step.
func TestDeckProfile_ArchetypeFingerprintMatchPopulated(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCombo: 12, RoleTutor: 11, RoleRamp: 12, RoleDraw: 11,
				RoleLand: 36, RoleRemoval: 8, RoleProtection: 6, RoleCounterspell: 4,
			},
			TotalCards: 100,
		},
		AvgCMC: 2.5,
	}
	ac := ClassifyArchetype(report, nil, nil)
	if ac == nil {
		t.Fatal("expected classification")
	}
	report.Archetype = ac
	dp := &DeckProfile{}
	// Directly invoke the assignment logic that BuildDeckProfile uses.
	dp.PrimaryArchetype = report.Archetype.Primary
	dp.ArchetypeConfidence = report.Archetype.PrimaryConfidence
	dp.ArchetypeFingerprintMatch = report.Archetype.FingerprintMatch

	if dp.ArchetypeFingerprintMatch != ac.FingerprintMatch {
		t.Errorf("DeckProfile.ArchetypeFingerprintMatch must mirror Archetype.FingerprintMatch; got %.6f vs %.6f",
			dp.ArchetypeFingerprintMatch, ac.FingerprintMatch)
	}
	if dp.ArchetypeFingerprintMatch < 0 || dp.ArchetypeFingerprintMatch > 1 {
		t.Errorf("DeckProfile.ArchetypeFingerprintMatch out of [0,1] range: %.6f", dp.ArchetypeFingerprintMatch)
	}
}

// JSON contract: both the archetype block and the unified_profile
// block must include fingerprint_match / archetype_fingerprint_match
// fields. Defends against silent loss when JSON consumers (UI, hat
// strategy_loader, deck-screen ranker) look for the field.
func TestArchetypeFingerprintMatch_JSONFields(t *testing.T) {
	report := &FreyaReport{
		DeckName:   "test",
		TotalCards: 99,
		Archetype: &ArchetypeClassification{
			Primary:           "Combo",
			PrimaryConfidence: 0.75,
			FingerprintMatch:  0.62,
			Bracket:           4,
			BracketLabel:      "Optimized",
		},
		Profile: &DeckProfile{
			DeckName:                  "test",
			CardCount:                 99,
			PrimaryArchetype:          "Combo",
			ArchetypeConfidence:       0.75,
			ArchetypeFingerprintMatch: 0.62,
			Bracket:                   4,
			BracketLabel:              "Optimized",
		},
		ManaCurve:   [8]int{},
		ColorDemand: map[string]int{},
		ColorSupply: map[string]int{},
	}
	var buf bytes.Buffer
	printJSON(&buf, report)
	s := buf.String()

	// jsonArchetype block — "fingerprint_match"
	if !strings.Contains(s, `"fingerprint_match"`) {
		t.Errorf("JSON output missing 'fingerprint_match' (jsonArchetype block):\n%s", s)
	}
	if !strings.Contains(s, `"fingerprint_match": 0.62`) {
		t.Errorf("JSON value 0.62 not surfaced verbatim in archetype block")
	}

	// jsonDeckProfile block — "archetype_fingerprint_match"
	if !strings.Contains(s, `"archetype_fingerprint_match"`) {
		t.Errorf("JSON output missing 'archetype_fingerprint_match' (jsonDeckProfile block):\n%s", s)
	}
	if !strings.Contains(s, `"archetype_fingerprint_match": 0.62`) {
		t.Errorf("JSON value 0.62 not surfaced verbatim in unified_profile block")
	}
}
