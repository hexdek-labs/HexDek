package main

import (
	"bytes"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Combo timing tests — assert EarliestTurn estimates against known
// combo shapes (turn-3 cEDH, turn-5 mid, turn-7 jank) and pin the
// pacing labels + BracketHint. Also unit-test the naturalManaTurn
// formula against hand-computed expected values.
// ---------------------------------------------------------------------------

func timingReport(profiles []CardProfile, combos [][]string, rampCount int,
	tutorCount int, keepablePct float64) *ComboTimingReport {

	r := &FreyaReport{Profiles: profiles, NonLandTutorCount: tutorCount}
	for _, cards := range combos {
		r.TrueInfinites = append(r.TrueInfinites, ComboResult{
			Cards: append([]string(nil), cards...), LoopType: "true_infinite",
		})
	}
	dp := &DeckProfile{
		RampCount:       rampCount,
		KeepableHandPct: keepablePct,
	}
	return BuildComboTimingReport(r, dp)
}

// TestTiming_NaturalManaTurnFormula: unit-test the core arithmetic.
func TestTiming_NaturalManaTurnFormula(t *testing.T) {
	cases := []struct {
		total, max, want int
		desc             string
	}{
		{0, 0, 1, "zero total → turn 1 placeholder"},
		{5, 3, 4, "Thoracle+Consultation shape: sum 5 max 3 → T=3 + 1 = 4"},
		{6, 3, 4, "Heliod+Ballista shape: sum 6 max 3 → T=3 + 1 = 4"},
		{8, 5, 6, "Karmic Guide+Reveillark: sum 8 max 5 → clamp T=5 + 1 = 6"},
		{12, 5, 6, "4-piece slow combo: sum 12 max 5 → T=5 (15>=12) + 1 = 6"},
		{2, 2, 3, "1-card combo at 2: T=2 (3≥2) + 1 = 3"},
		{1, 1, 2, "1-card combo at 1: T=1 (1≥1) clamp ≥1 + 1 = 2"},
	}
	for _, c := range cases {
		got := naturalManaTurn(c.total, c.max)
		if got != c.want {
			t.Errorf("%s: naturalManaTurn(%d,%d)=%d, want %d",
				c.desc, c.total, c.max, got, c.want)
		}
	}
}

// TestTiming_Turn3CEDH: turn-3 cEDH combo (Thoracle + Consultation
// shape) with heavy ramp + tutor compression. Natural turn 4, ramp 2
// compression, tutor 2 compression → 4-2-2=0 → floored to turn 2.
// Pacing "fast", BracketHint "B5 cEDH".
func TestTiming_Turn3CEDH(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Thassa's Oracle", CMC: 3, TypeLine: "Creature"},
		{Name: "Demonic Consultation", CMC: 2, TypeLine: "Instant"},
	}
	r := timingReport(profiles,
		[][]string{{"Thassa's Oracle", "Demonic Consultation"}},
		16, 8, 0.65)
	v := r.PerCombo[0]
	if v.TotalCMC != 5 || v.MaxPieceCMC != 3 {
		t.Errorf("TotalCMC/MaxPieceCMC: got %d/%d, want 5/3", v.TotalCMC, v.MaxPieceCMC)
	}
	if v.NaturalTurn != 4 {
		t.Errorf("NaturalTurn: got %d, want 4", v.NaturalTurn)
	}
	if v.RampCompression != 2 || v.TutorCompression != 2 {
		t.Errorf("Compressions: got ramp=%d tutor=%d, want 2/2",
			v.RampCompression, v.TutorCompression)
	}
	if v.EarliestTurn != 2 {
		t.Errorf("EarliestTurn: got %d, want 2 (floored)", v.EarliestTurn)
	}
	if v.Pacing != "fast" {
		t.Errorf("Pacing: got %q, want \"fast\"", v.Pacing)
	}
	if r.BracketHint != "B5 cEDH" {
		t.Errorf("BracketHint: got %q, want \"B5 cEDH\"", r.BracketHint)
	}
}

// TestTiming_Turn5Mid: mid-power combo (CMC 3+3=6) with moderate
// ramp (8 = compression 1) and no tutors → turn 4-1 = 3 (still cEDH
// territory). Cranking ramp to 0 + tutors to 0 → turn 4 → B4.
func TestTiming_Turn5Mid(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Engine A", CMC: 3, TypeLine: "Creature"},
		{Name: "Payoff B", CMC: 3, TypeLine: "Creature"},
	}
	// Moderate setup: 8 ramp = compression 1, no tutors.
	r := timingReport(profiles,
		[][]string{{"Engine A", "Payoff B"}}, 8, 0, 0.65)
	v := r.PerCombo[0]
	if v.NaturalTurn != 4 {
		t.Errorf("NaturalTurn: got %d, want 4", v.NaturalTurn)
	}
	if v.RampCompression != 1 || v.TutorCompression != 0 {
		t.Errorf("Compressions: got ramp=%d tutor=%d, want 1/0",
			v.RampCompression, v.TutorCompression)
	}
	if v.EarliestTurn != 3 {
		t.Errorf("EarliestTurn: got %d, want 3", v.EarliestTurn)
	}
	if v.Pacing != "fast" {
		t.Errorf("Pacing: got %q, want \"fast\"", v.Pacing)
	}
	// With zero ramp + zero tutors the same combo lands turn 4
	// (mid pacing, B4 high-power hint).
	r2 := timingReport(profiles,
		[][]string{{"Engine A", "Payoff B"}}, 0, 0, 0.65)
	if r2.PerCombo[0].EarliestTurn != 4 {
		t.Errorf("no-support EarliestTurn: got %d, want 4", r2.PerCombo[0].EarliestTurn)
	}
	if r2.PerCombo[0].Pacing != "mid" {
		t.Errorf("no-support Pacing: got %q, want \"mid\"", r2.PerCombo[0].Pacing)
	}
	if r2.BracketHint != "B4 high-power" {
		t.Errorf("no-support BracketHint: got %q, want \"B4 high-power\"", r2.BracketHint)
	}
}

// TestTiming_Turn7Jank: 4-piece slow combo (CMC 4+5+3+2=14, max 5).
// Natural turn = max(5, ceil) + 1 = solve T(T+1)/2 ≥ 14 → T=5 (15>=14)
// clamp ≥5 → +1 = 6. With no ramp / no tutors → turn 6 (slow, B2-B3).
// Adding hand penalty pushes to turn 7.
func TestTiming_Turn7Jank(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Big1", CMC: 5, TypeLine: "Creature"},
		{Name: "Big2", CMC: 4, TypeLine: "Creature"},
		{Name: "Mid", CMC: 3, TypeLine: "Enchantment"},
		{Name: "Cheap", CMC: 2, TypeLine: "Artifact"},
	}
	// Bad hand (< 0.50) → hand penalty +1.
	r := timingReport(profiles,
		[][]string{{"Big1", "Big2", "Mid", "Cheap"}}, 0, 0, 0.40)
	v := r.PerCombo[0]
	if v.NaturalTurn != 6 {
		t.Errorf("NaturalTurn: got %d, want 6", v.NaturalTurn)
	}
	if v.HandPenalty != 1 {
		t.Errorf("HandPenalty: got %d, want 1 (bad hand)", v.HandPenalty)
	}
	if v.EarliestTurn != 7 {
		t.Errorf("EarliestTurn: got %d, want 7 (6 + 1 hand)", v.EarliestTurn)
	}
	if v.Pacing != "slow" {
		t.Errorf("Pacing: got %q, want \"slow\"", v.Pacing)
	}
	if r.BracketHint != "B2-B3 mid/casual" {
		t.Errorf("BracketHint: got %q, want \"B2-B3 mid/casual\"", r.BracketHint)
	}
}

// TestTiming_RampCompressionCap: 24 ramp pieces still caps at 2
// compression (defensive against unrealistic claims).
func TestTiming_RampCompressionCap(t *testing.T) {
	profiles := []CardProfile{
		{Name: "A", CMC: 3, TypeLine: "Creature"},
		{Name: "B", CMC: 3, TypeLine: "Creature"},
	}
	r := timingReport(profiles, [][]string{{"A", "B"}}, 24, 0, 0.65)
	if r.PerCombo[0].RampCompression != 2 {
		t.Errorf("RampCompression: got %d, want 2 (capped)",
			r.PerCombo[0].RampCompression)
	}
}

// TestTiming_TutorCompressionCap: 16 tutors caps at 2 compression.
func TestTiming_TutorCompressionCap(t *testing.T) {
	profiles := []CardProfile{
		{Name: "A", CMC: 3, TypeLine: "Creature"},
		{Name: "B", CMC: 3, TypeLine: "Creature"},
	}
	r := timingReport(profiles, [][]string{{"A", "B"}}, 0, 16, 0.65)
	if r.PerCombo[0].TutorCompression != 2 {
		t.Errorf("TutorCompression: got %d, want 2 (capped)",
			r.PerCombo[0].TutorCompression)
	}
}

// TestTiming_InstantOnlyCombo: 2x instant pieces (Consultation +
// Tainted Pact shape). Natural turn computed off CMC just like
// permanents — no special treatment. CMC 2+1=3, max 2 → T=2 + 1 = 3.
func TestTiming_InstantOnlyCombo(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Demonic Consultation", CMC: 2, TypeLine: "Instant"},
		{Name: "Lim-Dul's Vault", CMC: 1, TypeLine: "Instant"},
	}
	r := timingReport(profiles,
		[][]string{{"Demonic Consultation", "Lim-Dul's Vault"}}, 0, 0, 0.65)
	v := r.PerCombo[0]
	if v.NaturalTurn != 3 {
		t.Errorf("NaturalTurn: got %d, want 3 (CMC 3 → T=2 + 1)", v.NaturalTurn)
	}
}

// TestTiming_MixedDeckRollup: 3 combos at distinct turns. Verify
// MinTurn/MaxTurn/MedianTurn + Fastest/Slowest combo identification.
func TestTiming_MixedDeckRollup(t *testing.T) {
	profiles := []CardProfile{
		{Name: "Fast1", CMC: 2, TypeLine: "Creature"},
		{Name: "Fast2", CMC: 2, TypeLine: "Creature"},
		{Name: "Mid1", CMC: 3, TypeLine: "Creature"},
		{Name: "Mid2", CMC: 3, TypeLine: "Creature"},
		{Name: "Slow1", CMC: 5, TypeLine: "Creature"},
		{Name: "Slow2", CMC: 4, TypeLine: "Creature"},
	}
	r := timingReport(profiles, [][]string{
		{"Fast1", "Fast2"}, // CMC 4, max 2 → T=2 (3>=4? no. T=3 (6>=4)) clamp ≥2 → T=3 + 1 = 4. With no ramp = turn 4
		{"Mid1", "Mid2"},   // CMC 6, max 3 → T=3 + 1 = 4
		{"Slow1", "Slow2"}, // CMC 9, max 5 → T=4 (10>=9) clamp ≥5 → +1 = 6
	}, 0, 0, 0.65)
	if r.MinTurn != 4 {
		t.Errorf("MinTurn: got %d, want 4", r.MinTurn)
	}
	if r.MaxTurn != 6 {
		t.Errorf("MaxTurn: got %d, want 6", r.MaxTurn)
	}
	if !strings.Contains(r.SlowestComboLabel, "Slow1") {
		t.Errorf("SlowestComboLabel: got %q, expected to contain Slow1", r.SlowestComboLabel)
	}
}

// TestTiming_NilOnNoCombos: defensive nil handling.
func TestTiming_NilOnNoCombos(t *testing.T) {
	if r := BuildComboTimingReport(nil, nil); r != nil {
		t.Errorf("expected nil for nil inputs, got %+v", r)
	}
	if r := BuildComboTimingReport(&FreyaReport{}, &DeckProfile{}); r != nil {
		t.Errorf("expected nil for combo-less report, got %+v", r)
	}
}

// TestTiming_ZeroCMCFallback: unmapped piece (not in Profiles) skipped
// silently — total stays 0 → naturalTurn = 1 → EarliestTurn = 2 (floor).
func TestTiming_ZeroCMCFallback(t *testing.T) {
	r := timingReport(nil, [][]string{{"Unknown Card A", "Unknown Card B"}}, 0, 0, 0.65)
	v := r.PerCombo[0]
	if v.TotalCMC != 0 {
		t.Errorf("TotalCMC: got %d, want 0 (unmapped pieces)", v.TotalCMC)
	}
	if v.EarliestTurn != 2 {
		t.Errorf("EarliestTurn: got %d, want 2 (floor)", v.EarliestTurn)
	}
}

// TestPrintComboTiming_RendersExpectedShape: text output contains
// canonical headers, per-combo turn lines, and bracket hint.
func TestPrintComboTiming_RendersExpectedShape(t *testing.T) {
	profiles := []CardProfile{
		{Name: "A", CMC: 3, TypeLine: "Creature"},
		{Name: "B", CMC: 3, TypeLine: "Creature"},
	}
	r := timingReport(profiles, [][]string{{"A", "B"}}, 8, 4, 0.65)
	var buf bytes.Buffer
	printComboTiming(&buf, r)
	out := buf.String()
	musts := []string{
		"COMBO TIMING",
		"bracket hint:",
		"Fastest combo:",
		"turn ",
		"(fast)",
		"natural",
	}
	for _, s := range musts {
		if !strings.Contains(out, s) {
			t.Errorf("text output missing %q\nfull:\n%s", s, out)
		}
	}
}

// TestPrintComboTiming_NilNoOp: nil input renders nothing.
func TestPrintComboTiming_NilNoOp(t *testing.T) {
	var buf bytes.Buffer
	printComboTiming(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("expected zero output for nil, got: %s", buf.String())
	}
}

// TestTiming_BracketHintBoundaries: pin bracket boundaries.
func TestTiming_BracketHintBoundaries(t *testing.T) {
	cases := []struct {
		minTurn int
		want    string
	}{
		{1, "B5 cEDH"},
		{2, "B5 cEDH"},
		{3, "B5 cEDH"},
		{4, "B4 high-power"},
		{5, "B4 high-power"},
		{6, "B2-B3 mid/casual"},
		{10, "B2-B3 mid/casual"},
	}
	for _, c := range cases {
		if got := bracketHintFromTurn(c.minTurn); got != c.want {
			t.Errorf("bracketHintFromTurn(%d): got %q, want %q", c.minTurn, got, c.want)
		}
	}
}

// TestTiming_PacingLabelBoundaries: pin pacing boundaries.
func TestTiming_PacingLabelBoundaries(t *testing.T) {
	cases := []struct {
		turn int
		want string
	}{
		{1, "fast"},
		{3, "fast"},
		{4, "mid"},
		{5, "mid"},
		{6, "slow"},
		{10, "slow"},
	}
	for _, c := range cases {
		if got := pacingLabel(c.turn); got != c.want {
			t.Errorf("pacingLabel(%d): got %q, want %q", c.turn, got, c.want)
		}
	}
}
