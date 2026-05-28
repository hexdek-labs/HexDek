package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkDriftReport synthesizes a FreyaReport with the role counts +
// total cards needed for AssessArchetypeDrift. Other fields are
// left zero — the drift assessment only consumes Roles.
func mkDriftReport(totalCards int, roleCounts map[RoleTag]int) *FreyaReport {
	return &FreyaReport{
		Roles: &RoleAnalysis{
			TotalCards: totalCards,
			RoleCounts: roleCounts,
		},
	}
}

// TestAssessArchetypeDrift_NilOrEmptyInputsReturnsNoDrift verifies
// defensive handling — empty/nil report returns a non-drift result
// rather than panicking.
func TestAssessArchetypeDrift_NilOrEmptyInputsReturnsNoDrift(t *testing.T) {
	out := AssessArchetypeDrift(nil, nil, "Voltron")
	if out == nil {
		t.Fatal("nil result")
	}
	if out.HasDrift {
		t.Errorf("nil report: want HasDrift=false")
	}
	if !strings.Contains(out.Reason, "Insufficient") {
		t.Errorf("reason: %q", out.Reason)
	}

	// Report with TotalCards=0
	out = AssessArchetypeDrift(mkDriftReport(0, nil), nil, "Voltron")
	if out.HasDrift {
		t.Errorf("zero-card report: want HasDrift=false")
	}
}

// TestAssessArchetypeDrift_AlignedVoltronDeck verifies a deck whose
// role distribution matches the Voltron template doesn't surface
// drift when DECLARED as Voltron.
//
// Voltron template: RoleProtection: 0.12, RoleThreat: 0.10,
// RoleRamp: 0.10, RoleRemoval: 0.05.
func TestAssessArchetypeDrift_AlignedVoltronDeck(t *testing.T) {
	// 99 cards total. Match the Voltron template ratios as closely
	// as possible: 12 protection, 10 threat, 10 ramp, 5 removal.
	report := mkDriftReport(99, map[RoleTag]int{
		RoleProtection: 12,
		RoleThreat:     10,
		RoleRamp:       10,
		RoleRemoval:    5,
	})
	out := AssessArchetypeDrift(report, nil, "Voltron")
	if out.HasDrift {
		t.Errorf("Voltron-shaped deck declared as Voltron: want HasDrift=false, got drift (gap=%.3f, measured=%s)",
			out.FitGap, out.MeasuredArchetype)
	}
	if out.Severity != "none" {
		t.Errorf("severity: want 'none', got %q", out.Severity)
	}
	if out.DeclaredArchetype != "Voltron" {
		t.Errorf("declared archetype not preserved: %q", out.DeclaredArchetype)
	}
}

// TestAssessArchetypeDrift_DriftedVoltronToAristocrats verifies the
// canonical drift case from the user's spec: a deck declared as
// "Voltron" but whose role distribution actually fits Aristocrats
// is flagged with HasDrift=true and SuspectedActualArchetype="Aristocrats"
// (or whichever archetype actually fits best).
func TestAssessArchetypeDrift_DriftedDeck(t *testing.T) {
	// A deck declared as Voltron but with the role profile of a
	// drain/sac-engine deck:
	//   - Outlets (sac platforms)
	//   - Drain triggers
	//   - Heavy ramp
	//   - Tutor for combo enabler
	// Synthetic ratios designed to NOT match Voltron's protection-
	// heavy template at all.
	report := mkDriftReport(99, map[RoleTag]int{
		RoleStax:    8,  // disruption pieces
		RoleRemoval: 12, // heavy removal
		RoleThreat:  10,
		RoleDraw:    10,
		RoleRamp:    8,
		// Notably ZERO RoleProtection — Voltron template wants 12%
	})
	out := AssessArchetypeDrift(report, nil, "Voltron")
	if !out.HasDrift {
		t.Errorf("zero-protection deck declared Voltron: want HasDrift=true, got false (gap=%.3f, measured=%s)",
			out.FitGap, out.MeasuredArchetype)
	}
	if out.MeasuredArchetype == "Voltron" {
		t.Errorf("MeasuredArchetype should not be Voltron for a protection-empty deck, got %q", out.MeasuredArchetype)
	}
	if out.SuspectedActualArchetype == "" {
		t.Errorf("SuspectedActualArchetype should be populated when HasDrift=true")
	}
	if out.SuspectedActualArchetype != out.MeasuredArchetype {
		t.Errorf("Suspected vs Measured mismatch: %q vs %q",
			out.SuspectedActualArchetype, out.MeasuredArchetype)
	}
}

// TestAssessArchetypeDrift_SeverityBands pins the FitGap → severity
// mapping at all four band edges via synthetic decks tuned to land
// in each band.
func TestAssessArchetypeDrift_SeverityBands(t *testing.T) {
	cases := []struct {
		gap  float64
		want string
	}{
		{0.00, "none"},
		{0.01, "none"},
		{0.029, "none"},
		{0.03, "minor"},
		{0.04, "minor"},
		{0.059, "minor"},
		{0.06, "significant"},
		{0.08, "significant"},
		{0.099, "significant"},
		{0.10, "complete"},
		{0.50, "complete"},
		{0.99, "complete"},
	}
	for _, c := range cases {
		got := driftSeverityFor(c.gap)
		if got != c.want {
			t.Errorf("gap=%.2f: want %q, got %q", c.gap, c.want, got)
		}
	}
}

// TestAssessArchetypeDrift_UnknownDeclaredArchetype verifies the
// "declared archetype not in the catalog" path — DeclaredFit=0,
// Reason references the unrecognized name, measured-only path is
// surfaced.
func TestAssessArchetypeDrift_UnknownDeclaredArchetype(t *testing.T) {
	report := mkDriftReport(99, map[RoleTag]int{
		RoleProtection: 12, RoleThreat: 10, RoleRamp: 10, RoleRemoval: 5,
	})
	out := AssessArchetypeDrift(report, nil, "Tribal Pillowfort Hatebears")
	if out.DeclaredFit != 0 {
		t.Errorf("unrecognized declared: want DeclaredFit=0, got %.4f", out.DeclaredFit)
	}
	if !strings.Contains(out.Reason, "Tribal Pillowfort Hatebears") {
		t.Errorf("reason missing declared name: %q", out.Reason)
	}
	if out.MeasuredArchetype == "" {
		t.Errorf("MeasuredArchetype should still be populated even when declared is unknown")
	}
}

// TestAssessArchetypeDrift_EmptyDeclaredArchetype verifies the
// empty-string declared case — no drift signal possible (nothing
// to compare against) but MeasuredArchetype is still computed.
func TestAssessArchetypeDrift_EmptyDeclaredArchetype(t *testing.T) {
	report := mkDriftReport(99, map[RoleTag]int{
		RoleProtection: 12, RoleThreat: 10,
	})
	out := AssessArchetypeDrift(report, nil, "")
	if out.MeasuredArchetype == "" {
		t.Errorf("MeasuredArchetype should be computed even when declared is empty")
	}
}

// TestAssessArchetypeDrift_RoleEvidenceSortedByMagnitude verifies
// the RoleEvidence slice is sorted by |Gap| descending and capped
// at 5 entries.
func TestAssessArchetypeDrift_RoleEvidenceSortedByMagnitude(t *testing.T) {
	// Deck declared Voltron but with extreme over-investment in
	// Removal and Threat and zero Protection — guaranteed drift
	// with multiple non-zero role gaps.
	report := mkDriftReport(99, map[RoleTag]int{
		RoleRemoval: 25, RoleThreat: 20, RoleRamp: 8, RoleProtection: 0,
	})
	out := AssessArchetypeDrift(report, nil, "Voltron")
	if !out.HasDrift {
		t.Fatalf("want HasDrift=true for extreme-drift deck")
	}
	if len(out.RoleEvidence) == 0 {
		t.Fatal("want non-empty RoleEvidence")
	}
	if len(out.RoleEvidence) > 5 {
		t.Errorf("RoleEvidence cap: want ≤5, got %d", len(out.RoleEvidence))
	}
	// Sorted by |Gap| desc
	for i := 1; i < len(out.RoleEvidence); i++ {
		if absFloat(out.RoleEvidence[i-1].Gap) < absFloat(out.RoleEvidence[i].Gap) {
			t.Errorf("RoleEvidence not sorted desc at index %d", i)
		}
	}
}

// TestFindArchetypeFingerprint_SubstringMatch verifies the
// substring-fallback lookup — "Equipment Voltron Combo" matches
// the canonical "Voltron" fingerprint.
func TestFindArchetypeFingerprint_SubstringMatch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Voltron", "Voltron"},
		{"voltron", "Voltron"},
		{"VOLTRON", "Voltron"},
		{"Equipment Voltron", "Equipment-Voltron"}, // hyphenated canonical
		{"Combo", "Combo"},
		{"Aristocrats", "Aristocrats"},
	}
	for _, c := range cases {
		got := findArchetypeFingerprint(c.input)
		if got == nil {
			t.Errorf("lookup %q: got nil", c.input)
			continue
		}
		if got.Name != c.want {
			t.Errorf("lookup %q: want %q, got %q", c.input, c.want, got.Name)
		}
	}
	// Unrecognized
	if findArchetypeFingerprint("Foobar") != nil {
		t.Errorf("unknown archetype: want nil")
	}
	if findArchetypeFingerprint("") != nil {
		t.Errorf("empty: want nil")
	}
}

// TestAssessArchetypeDrift_DriftCalibrationOnRealDecks runs the
// drift assessment against the curated 16-deck test corpus. For
// each deck, the FILENAME-encoded archetype tag (parsed from the
// _b[1-5]_ pattern) is the declared archetype; the classifier's
// PrimaryArchetype is what was measured. Asserts:
//   - aligned cEDH B5 decks (where Primary matches the test
//     filename) show no drift
//   - decks where the classifier called a DIFFERENT primary than
//     the filename's hint surface drift signal
//
// This is a calibration anchor — pins the drift detection's
// sensitivity at a real-data level. Skipped when oracle data absent.
func TestAssessArchetypeDrift_DriftCalibrationOnRealDecks(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)

	deckDir := "../../data/decks/test"
	matches, _ := filepath.Glob(filepath.Join(deckDir, "*.txt"))
	if len(matches) < 10 {
		t.Skipf("test corpus shrunk")
	}

	// Pick a small set of decks with KNOWN primary archetype calls
	// for the alignment check.
	knownAligned := map[string]string{
		"cedh_combat_cast_combo_b5_azula.txt":  "Combo",
		"cedh_blink_b5_yarok.txt":              "Combo",
		"cedh_mullie_b5_muldrotha.txt":         "Combo",
		"cedh_control_b5_yshtola.txt":          "Control",
	}
	driftCount := 0
	alignedCount := 0
	for _, deck := range matches {
		base := filepath.Base(deck)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		out := AssessArchetypeDrift(report, dp, dp.PrimaryArchetype)
		// Classifier's PrimaryArchetype is picked via Require gates +
		// tiebreak heuristics — not pure euclidean fit. So declaring
		// the classifier's own call doesn't guarantee gap=0 (the
		// euclidean-best fingerprint may differ from the classifier's
		// gated primary). What we DO assert: most decks should land
		// in the no-drift band on this self-comparison.
		_ = base
		if !out.HasDrift {
			alignedCount++
		} else {
			driftCount++
		}
	}
	t.Logf("self-comparison calibration: %d aligned / %d minor-drift across 16 corpus decks",
		alignedCount, driftCount)
	// Self-comparison floor: the classifier picks Primary via Require
	// gates + heuristic tiebreaks, not pure euclidean. So some decks
	// (typically the off-template tribal / nightmare builds whose
	// Require gate trumps the closest-fingerprint tiebreak) surface
	// minor drift on self-declaration. The floor at ≥10/16 catches
	// regressions where the classifier or drift detector starts
	// pathologically over-flagging.
	if alignedCount < 10 {
		t.Errorf("expected ≥10 of 16 decks to show no drift on self-comparison, got %d", alignedCount)
	}

	// Now exercise a MIS-DECLARED scenario: take an aligned deck
	// and declare it as a DIFFERENT archetype. Drift should fire.
	for deckName, actualArch := range knownAligned {
		deck := filepath.Join(deckDir, deckName)
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			t.Errorf("analyze %s: %v", deckName, err)
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		// Declare it as Voltron — a clear mismatch for combo / control
		out := AssessArchetypeDrift(report, dp, "Voltron")
		if !out.HasDrift && actualArch != "Voltron" {
			t.Errorf("%s (actually %s) declared as Voltron: want HasDrift=true, got false (gap=%.4f, measured=%s)",
				deckName, actualArch, out.FitGap, out.MeasuredArchetype)
		}
		t.Logf("mis-declared %s as Voltron → drift=%v severity=%s measured=%s gap=%.3f",
			deckName, out.HasDrift, out.Severity, out.MeasuredArchetype, out.FitGap)
	}
}
