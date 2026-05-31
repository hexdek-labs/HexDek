package main

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestComputeMetrics_Happy exercises ComputeMetrics against a hand-built
// FreyaReport with known shapes for all three percentages. The fixture
// has:
//   - Archetype primary with Confidence=0.85 → counts as high
//   - 3 WinLines with confidences 0.9 / 0.4 / 0.7 → 1 high
//   - 5 cards: 3 with explicit non-land tags, 1 land-only (excluded),
//     1 unclassified (counted but not "explicit-tagged")
//   - 4 combos: 3 with non-empty Description, 1 with empty Description
//     but populated Annotation.Summary → counts as having summary
//     (4 total, 4 with summary)
//
// Expected:
//   - ConfidenceItemCount = 4 (1 archetype + 3 winlines)
//   - ConfidenceHighCount = 2 (archetype 0.85, winline 0.9)
//   - ConfidenceHighPct = 50.0
//   - CardTagCount = 4 (5 minus the land-only)
//   - CardWithExplicitTagCount = 3
//   - CardWithExplicitTagPct = 75.0
//   - ComboCount = 4
//   - ComboWithSummaryCount = 4
//   - ComboWithSummaryPct = 100.0
func TestComputeMetrics_Happy(t *testing.T) {
	report := &FreyaReport{
		DeckName:   "test-deck",
		DeckPath:   "/tmp/test.txt",
		TotalCards: 100,
		Archetype: &ArchetypeClassification{
			Primary:           "Combo",
			PrimaryConfidence: 0.85,
		},
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{
				{Pieces: []string{"A", "B"}, Confidence: 0.9},
				{Pieces: []string{"C", "D"}, Confidence: 0.4},
				{Pieces: []string{"E", "F"}, Confidence: 0.7},
			},
		},
		Roles: &RoleAnalysis{
			Assignments: []CardRoleAssignment{
				{Name: "Sol Ring", Roles: []RoleTag{RoleRamp}},
				{Name: "Counterspell", Roles: []RoleTag{RoleCounterspell}},
				{Name: "Demonic Tutor", Roles: []RoleTag{RoleTutor}},
				{Name: "Forest", Roles: []RoleTag{RoleLand}},     // excluded
				{Name: "Mystery Card", Roles: []RoleTag{}},        // counted, not explicit
			},
		},
		TrueInfinites: []ComboResult{
			{Cards: []string{"Thoracle", "Consultation"}, Description: "Thassa's Oracle + Demonic Consultation: exile library then trigger ETB win"},
			{Cards: []string{"Heliod", "Ballista"}, Description: "Heliod, Sun-Crowned + Walking Ballista: infinite damage"},
		},
		Determined: []ComboResult{
			{Cards: []string{"Aggravated Assault", "Bear Umbra"}, Description: "infinite combat with the right mana"},
		},
		Finishers: []ComboResult{
			// Empty Description but Annotation.Summary populated — should
			// still count as "has human summary".
			{Cards: []string{"X", "Y", "Z"}, Description: "", Annotation: &LoopAnnotation{Summary: "5-card grindy value engine into infinite mana"}},
		},
	}

	m := ComputeMetrics(report)

	if m.DeckName != "test-deck" {
		t.Errorf("DeckName = %q, want %q", m.DeckName, "test-deck")
	}
	if m.TotalCards != 100 {
		t.Errorf("TotalCards = %d, want 100", m.TotalCards)
	}

	if m.ConfidenceItemCount != 4 {
		t.Errorf("ConfidenceItemCount = %d, want 4 (1 archetype + 3 winlines)", m.ConfidenceItemCount)
	}
	if m.ConfidenceHighCount != 2 {
		t.Errorf("ConfidenceHighCount = %d, want 2 (archetype 0.85, winline 0.9)", m.ConfidenceHighCount)
	}
	if !floatNear(m.ConfidenceHighPct, 50.0) {
		t.Errorf("ConfidenceHighPct = %f, want ~50.0", m.ConfidenceHighPct)
	}

	if m.CardTagCount != 4 {
		t.Errorf("CardTagCount = %d, want 4 (5 minus 1 land-only)", m.CardTagCount)
	}
	if m.CardWithExplicitTagCount != 3 {
		t.Errorf("CardWithExplicitTagCount = %d, want 3", m.CardWithExplicitTagCount)
	}
	if !floatNear(m.CardWithExplicitTagPct, 75.0) {
		t.Errorf("CardWithExplicitTagPct = %f, want ~75.0", m.CardWithExplicitTagPct)
	}

	if m.ComboCount != 4 {
		t.Errorf("ComboCount = %d, want 4", m.ComboCount)
	}
	if m.ComboWithSummaryCount != 4 {
		t.Errorf("ComboWithSummaryCount = %d, want 4 (3 descriptions + 1 annotation summary)", m.ComboWithSummaryCount)
	}
	if !floatNear(m.ComboWithSummaryPct, 100.0) {
		t.Errorf("ComboWithSummaryPct = %f, want 100.0", m.ComboWithSummaryPct)
	}
}

// TestComputeMetrics_EmptyReport pins the zero-denominator path. An
// empty report should produce zero counts and zero percentages without
// dividing by zero.
func TestComputeMetrics_EmptyReport(t *testing.T) {
	report := &FreyaReport{DeckName: "empty"}
	m := ComputeMetrics(report)

	if m.ConfidenceItemCount != 0 || m.ConfidenceHighPct != 0 {
		t.Errorf("empty report should have zero confidence stats; got count=%d pct=%f",
			m.ConfidenceItemCount, m.ConfidenceHighPct)
	}
	if m.CardTagCount != 0 || m.CardWithExplicitTagPct != 0 {
		t.Errorf("empty report should have zero tag stats; got count=%d pct=%f",
			m.CardTagCount, m.CardWithExplicitTagPct)
	}
	if m.ComboCount != 0 || m.ComboWithSummaryPct != 0 {
		t.Errorf("empty report should have zero combo stats; got count=%d pct=%f",
			m.ComboCount, m.ComboWithSummaryPct)
	}
}

// TestComputeMetrics_NoArchetypePrimary verifies the archetype gate.
// An ArchetypeClassification with empty Primary should NOT count
// toward ConfidenceItemCount — Confidence is undefined when there's no
// classification call to score.
func TestComputeMetrics_NoArchetypePrimary(t *testing.T) {
	report := &FreyaReport{
		Archetype: &ArchetypeClassification{
			Primary:           "",
			PrimaryConfidence: 0.99, // would-be-high, but no Primary
		},
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{{Confidence: 0.95}},
		},
	}
	m := ComputeMetrics(report)
	if m.ConfidenceItemCount != 1 {
		t.Errorf("want 1 item (just the winline), got %d", m.ConfidenceItemCount)
	}
	if m.ConfidenceHighCount != 1 {
		t.Errorf("want 1 high (just the winline), got %d", m.ConfidenceHighCount)
	}
}

// TestComputeMetrics_LandOnlyExcluded pins the land-only exclusion.
// 10 lands should not contribute to CardTagCount (basic-land coverage
// isn't a Freya quality signal).
func TestComputeMetrics_LandOnlyExcluded(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			Assignments: []CardRoleAssignment{
				{Name: "Forest 1", Roles: []RoleTag{RoleLand}},
				{Name: "Forest 2", Roles: []RoleTag{RoleLand}},
				{Name: "Forest 3", Roles: []RoleTag{RoleLand}},
				{Name: "Llanowar Elves", Roles: []RoleTag{RoleRamp}},
			},
		},
	}
	m := ComputeMetrics(report)
	if m.CardTagCount != 1 {
		t.Errorf("CardTagCount = %d, want 1 (3 lands excluded)", m.CardTagCount)
	}
	if m.CardWithExplicitTagCount != 1 {
		t.Errorf("CardWithExplicitTagCount = %d, want 1", m.CardWithExplicitTagCount)
	}
}

// TestComputeMetrics_UnclassifiedCardCounted verifies the inverse: a
// card with ZERO roles is counted in the denominator but not the
// numerator. The point of the metric is to surface those — Freya
// classifier gaps.
func TestComputeMetrics_UnclassifiedCardCounted(t *testing.T) {
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			Assignments: []CardRoleAssignment{
				{Name: "Sol Ring", Roles: []RoleTag{RoleRamp}},
				{Name: "Unknown Card", Roles: nil}, // gap
			},
		},
	}
	m := ComputeMetrics(report)
	if m.CardTagCount != 2 {
		t.Errorf("CardTagCount = %d, want 2 (unclassified counted)", m.CardTagCount)
	}
	if m.CardWithExplicitTagCount != 1 {
		t.Errorf("CardWithExplicitTagCount = %d, want 1", m.CardWithExplicitTagCount)
	}
	if !floatNear(m.CardWithExplicitTagPct, 50.0) {
		t.Errorf("CardWithExplicitTagPct = %f, want 50.0", m.CardWithExplicitTagPct)
	}
}

// TestIsLandOnly pins the land-only helper. Empty roles slices are
// explicitly NOT land-only (they're unclassified gaps).
func TestIsLandOnly(t *testing.T) {
	cases := []struct {
		name  string
		roles []RoleTag
		want  bool
	}{
		{"empty", nil, false},
		{"land only", []RoleTag{RoleLand}, true},
		{"land plus ramp", []RoleTag{RoleLand, RoleRamp}, false},
		{"land plus utility", []RoleTag{RoleLand, RoleUtility}, false},
		{"ramp only", []RoleTag{RoleRamp}, false},
		{"three roles, no land", []RoleTag{RoleRamp, RoleDraw, RoleTutor}, false},
	}
	for _, c := range cases {
		got := isLandOnly(c.roles)
		if got != c.want {
			t.Errorf("isLandOnly(%v) = %v, want %v", c.roles, got, c.want)
		}
	}
}

// TestHasHumanSummary verifies the combo-summary predicate. Either a
// non-empty Description OR a non-empty Annotation.Summary counts;
// whitespace-only strings do not.
func TestHasHumanSummary(t *testing.T) {
	cases := []struct {
		name string
		c    ComboResult
		want bool
	}{
		{"empty everything", ComboResult{}, false},
		{"description set", ComboResult{Description: "real text"}, true},
		{"whitespace description", ComboResult{Description: "   \t\n"}, false},
		{"annotation summary only", ComboResult{Annotation: &LoopAnnotation{Summary: "loop text"}}, true},
		{"annotation present but empty summary", ComboResult{Annotation: &LoopAnnotation{Summary: ""}}, false},
		{"both populated", ComboResult{Description: "x", Annotation: &LoopAnnotation{Summary: "y"}}, true},
	}
	for _, c := range cases {
		got := hasHumanSummary(c.c)
		if got != c.want {
			t.Errorf("%s: hasHumanSummary = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestFirstLineDiff exercises the JSON-diff line locator used by the
// consistency probe. Covers byte-equal, single-line diff, length
// mismatch (one side shorter), and empty inputs.
func TestFirstLineDiff(t *testing.T) {
	cases := []struct {
		name        string
		a, b        string
		wantLine    int
		wantA, wantB string
	}{
		{
			name:     "byte equal",
			a:        "alpha\nbeta\ngamma",
			b:        "alpha\nbeta\ngamma",
			wantLine: 0,
		},
		{
			name:     "diff at line 2",
			a:        "alpha\nbeta\ngamma",
			b:        "alpha\nbeta-changed\ngamma",
			wantLine: 2, wantA: "beta", wantB: "beta-changed",
		},
		{
			name:     "diff at first line",
			a:        "alpha\nbeta",
			b:        "alpha-changed\nbeta",
			wantLine: 1, wantA: "alpha", wantB: "alpha-changed",
		},
		{
			name:     "a is prefix of b (a shorter)",
			a:        "alpha\nbeta",
			b:        "alpha\nbeta\ngamma",
			wantLine: 3, wantA: "", wantB: "gamma",
		},
		{
			name:     "b is prefix of a (b shorter)",
			a:        "alpha\nbeta\ngamma",
			b:        "alpha\nbeta",
			wantLine: 3, wantA: "gamma", wantB: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			line, a, b := firstLineDiff([]byte(c.a), []byte(c.b))
			if line != c.wantLine {
				t.Errorf("line = %d, want %d", line, c.wantLine)
			}
			if a != c.wantA {
				t.Errorf("a line = %q, want %q", a, c.wantA)
			}
			if b != c.wantB {
				t.Errorf("b line = %q, want %q", b, c.wantB)
			}
		})
	}
}

// TestPrintMetricsReport_Text smoke-tests the text renderer. Verifies
// each metric line is present and the bytes-printed matches the
// fixture's expected substrings. We don't pin the exact whitespace
// because future formatting tweaks are fine.
func TestPrintMetricsReport_Text(t *testing.T) {
	m := &FreyaMetrics{
		DeckName:                 "demo",
		DeckPath:                 "/tmp/demo.txt",
		TotalCards:               100,
		ConfidenceItemCount:      4,
		ConfidenceHighCount:      2,
		ConfidenceHighPct:        50.0,
		CardTagCount:             80,
		CardWithExplicitTagCount: 78,
		CardWithExplicitTagPct:   97.5,
		ComboCount:               5,
		ComboWithSummaryCount:    5,
		ComboWithSummaryPct:      100.0,
		Consistency: &ConsistencyResult{
			JSONByteEqual: true,
			Run1Bytes:     12345,
			Run2Bytes:     12345,
		},
	}
	var buf bytes.Buffer
	PrintMetricsReport(&buf, m, "text")
	out := buf.String()
	for _, want := range []string{
		"FREYA -- Output Quality Metrics",
		"demo",
		"Confidence coverage",
		"50.0%",
		"Cards with explicit non-land tag",
		"97.5%",
		"Combos with human-readable summary",
		"100.0%",
		"[OK] bit-identical",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

// TestPrintMetricsReport_JSON pins the JSON shape. The encoded blob
// must round-trip back into a FreyaMetrics with the same per-field
// values.
func TestPrintMetricsReport_JSON(t *testing.T) {
	m := &FreyaMetrics{
		DeckName:                 "demo",
		TotalCards:               100,
		ConfidenceHighCount:      3,
		ConfidenceItemCount:      5,
		ConfidenceHighPct:        60.0,
		CardWithExplicitTagCount: 75,
		CardTagCount:             80,
		CardWithExplicitTagPct:   93.75,
		ComboWithSummaryCount:    2,
		ComboCount:               3,
		ComboWithSummaryPct:      66.66666666666666,
	}
	var buf bytes.Buffer
	PrintMetricsReport(&buf, m, "json")
	var back FreyaMetrics
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("unmarshal JSON output: %v\n%s", err, buf.String())
	}
	if back.DeckName != m.DeckName {
		t.Errorf("DeckName round-trip: got %q, want %q", back.DeckName, m.DeckName)
	}
	if back.ConfidenceHighCount != m.ConfidenceHighCount {
		t.Errorf("ConfidenceHighCount round-trip mismatch")
	}
	if !floatNear(back.ComboWithSummaryPct, m.ComboWithSummaryPct) {
		t.Errorf("ComboWithSummaryPct round-trip drift: %f vs %f", back.ComboWithSummaryPct, m.ComboWithSummaryPct)
	}
}

// TestPrintMetricsReport_DriftRendering pins the "[DRIFT]" path of the
// consistency renderer. When the probe surfaces a non-determinism, the
// text output must name the first diverging line so the next
// investigator knows where to look.
func TestPrintMetricsReport_DriftRendering(t *testing.T) {
	m := &FreyaMetrics{
		DeckName:   "drift-demo",
		TotalCards: 100,
		Consistency: &ConsistencyResult{
			JSONByteEqual: false,
			FirstDiffLine: 42,
			FirstDiffRun1: `      "name": "alpha",`,
			FirstDiffRun2: `      "name": "beta",`,
			Run1Bytes:     12345,
			Run2Bytes:     12346,
		},
	}
	var buf bytes.Buffer
	PrintMetricsReport(&buf, m, "text")
	out := buf.String()
	for _, want := range []string{
		"[DRIFT]",
		"line 42",
		"alpha",
		"beta",
		"12345",
		"12346",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DRIFT output missing %q\n---\n%s", want, out)
		}
	}
}

func floatNear(a, b float64) bool { return math.Abs(a-b) < 0.001 }
