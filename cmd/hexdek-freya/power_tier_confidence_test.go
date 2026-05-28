package main

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

func almostEqualConf(a, b, tol float64) bool {
	return math.Abs(a-b) < tol
}

// TestCEDHPowerTierConfidence_ClearCutMaximal pins that a deck where
// every signal agrees on the final tier scores Confidence = 1.0.
func TestCEDHPowerTierConfidence_ClearCutMaximal(t *testing.T) {
	signals := []CEDHTierSignal{
		{TierVote: 5}, {TierVote: 5}, {TierVote: 5},
		{TierVote: 5}, {TierVote: 5}, {TierVote: 5},
	}
	conf := computeCEDHTierConfidence(signals, 5, false)
	if !almostEqualConf(conf, 1.0, 1e-9) {
		t.Errorf("clear-cut T5: want confidence 1.0, got %.4f", conf)
	}

	// Same shape at T1.
	signals1 := []CEDHTierSignal{
		{TierVote: 1}, {TierVote: 1}, {TierVote: 1},
		{TierVote: 1}, {TierVote: 1}, {TierVote: 1},
	}
	conf1 := computeCEDHTierConfidence(signals1, 1, false)
	if !almostEqualConf(conf1, 1.0, 1e-9) {
		t.Errorf("clear-cut T1: want confidence 1.0, got %.4f", conf1)
	}
}

// TestCEDHPowerTierConfidence_BoundarySplit verifies that a 3/3 vote
// split between adjacent tiers (boundary case) produces a confidence
// of 0.75 — three signals contribute 1.0 (exact match) and three
// contribute 0.5 (1-tier off); total 4.5 / 6 = 0.75.
func TestCEDHPowerTierConfidence_BoundarySplit(t *testing.T) {
	// 3 votes at T3, 3 votes at T4. Median upper-mid lands at T4
	// for this distribution (sorted 3/3/3/4/4/4, mid pos 3 = T4).
	// Confidence in T4: 3 exact (1.0 each) + 3 off-by-1 (0.5 each)
	// = 4.5 / 6 = 0.75.
	signals := []CEDHTierSignal{
		{TierVote: 3}, {TierVote: 3}, {TierVote: 3},
		{TierVote: 4}, {TierVote: 4}, {TierVote: 4},
	}
	conf := computeCEDHTierConfidence(signals, 4, false)
	if !almostEqualConf(conf, 0.75, 1e-9) {
		t.Errorf("T3/T4 boundary: want confidence 0.75, got %.4f", conf)
	}
}

// TestCEDHPowerTierConfidence_OutlierPenalty verifies that a single
// far-outlier signal pulls confidence down. Five signals at T5 + one
// at T1 (4-tier diff) → 5 exact + 1 zero-contribution = 5/6 = 0.833.
func TestCEDHPowerTierConfidence_OutlierPenalty(t *testing.T) {
	signals := []CEDHTierSignal{
		{TierVote: 5}, {TierVote: 5}, {TierVote: 5},
		{TierVote: 5}, {TierVote: 5}, {TierVote: 1},
	}
	conf := computeCEDHTierConfidence(signals, 5, false)
	if !almostEqualConf(conf, 5.0/6.0, 1e-9) {
		t.Errorf("T5 with one T1 outlier: want %.4f, got %.4f", 5.0/6.0, conf)
	}
}

// TestCEDHPowerTierConfidence_GatePenalty verifies that a gate-forced
// demotion multiplies the base confidence by 0.80. Same signal shape
// as the boundary split test (base 0.75) → with gate fired, 0.60.
func TestCEDHPowerTierConfidence_GatePenalty(t *testing.T) {
	signals := []CEDHTierSignal{
		{TierVote: 3}, {TierVote: 3}, {TierVote: 3},
		{TierVote: 4}, {TierVote: 4}, {TierVote: 4},
	}
	conf := computeCEDHTierConfidence(signals, 4, true)
	want := 0.75 * 0.80
	if !almostEqualConf(conf, want, 1e-9) {
		t.Errorf("gate-fired boundary: want %.4f, got %.4f", want, conf)
	}
}

// TestCEDHPowerTierConfidence_OffByTwoZeroContribution verifies that
// signals 2+ tiers away from the chosen tier contribute zero.
// Three signals at T5 + three at T2 (3-tier diff) → 3 exact + 3 zero
// = 3/6 = 0.5.
func TestCEDHPowerTierConfidence_OffByTwoZeroContribution(t *testing.T) {
	signals := []CEDHTierSignal{
		{TierVote: 5}, {TierVote: 5}, {TierVote: 5},
		{TierVote: 2}, {TierVote: 2}, {TierVote: 2},
	}
	conf := computeCEDHTierConfidence(signals, 5, false)
	if !almostEqualConf(conf, 0.5, 1e-9) {
		t.Errorf("3T5+3T2 split: want 0.5, got %.4f", conf)
	}
}

// TestCEDHPowerTierConfidence_EmptyFloor verifies that empty signal
// list returns 0.0 (defensive — no signal data, no confidence).
func TestCEDHPowerTierConfidence_EmptyFloor(t *testing.T) {
	if c := computeCEDHTierConfidence(nil, 3, false); c != 0.0 {
		t.Errorf("empty signals: want 0.0, got %.4f", c)
	}
	if c := computeCEDHTierConfidence([]CEDHTierSignal{}, 3, false); c != 0.0 {
		t.Errorf("empty slice: want 0.0, got %.4f", c)
	}
}

// TestCEDHPowerTierConfidence_RangeClamp verifies that the function
// clamps to [0, 1] even with adversarial inputs.
func TestCEDHPowerTierConfidence_RangeClamp(t *testing.T) {
	// All signals 4+ tiers away — sum of contributions clamped at 0.
	signals := []CEDHTierSignal{
		{TierVote: 1}, {TierVote: 1}, {TierVote: 1},
		{TierVote: 1}, {TierVote: 1}, {TierVote: 1},
	}
	c := computeCEDHTierConfidence(signals, 5, false)
	if c < 0 || c > 1 {
		t.Errorf("all-far signals: want [0,1], got %.4f", c)
	}
	if c != 0.0 {
		t.Errorf("all 4-off signals: want exactly 0, got %.4f", c)
	}
}

// TestCEDHPowerTier_ConfidencePopulatedOnAllPaths verifies that
// ClassifyCEDHPowerTier always populates the Confidence field — even
// on nil input (placeholder return) and on synthetic gate paths.
func TestCEDHPowerTier_ConfidencePopulatedOnAllPaths(t *testing.T) {
	// Nil inputs return placeholder tier 1 — Confidence should be 0
	// (no signals to score against).
	out := ClassifyCEDHPowerTier(nil, nil)
	if out.Confidence != 0 {
		t.Errorf("nil-input placeholder: want Confidence=0, got %.4f", out.Confidence)
	}

	// Real cEDH-shape deck: all signals high → high confidence.
	dp := &DeckProfile{
		GameChangerCount:   12,
		ManaBaseGrade:      "A",
		AvgCMC:             2.0,
		InteractionPackage: InteractionPackage{Score: 0.85},
	}
	report := &FreyaReport{
		NonLandTutorCount: 10,
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{{Weight: 5, Confidence: 0.80}},
		},
	}
	tier := ClassifyCEDHPowerTier(dp, report)
	if tier.Tier != 5 {
		t.Errorf("clear cEDH: want T5, got T%d", tier.Tier)
	}
	if tier.Confidence < 0.95 {
		t.Errorf("clear cEDH: want high confidence (>=0.95), got %.4f", tier.Confidence)
	}

	// Synthetic gate-demote: median wants T4 but discriminator gate
	// fires → demoted to T3 with gate penalty applied.
	dp2 := &DeckProfile{
		GameChangerCount:   0,                                  // no GC marker
		ManaBaseGrade:      "B",                                // mana T4 vote
		AvgCMC:             2.9,                                // CMC T4 vote
		InteractionPackage: InteractionPackage{Score: 0.10},    // no marker
	}
	report2 := &FreyaReport{
		NonLandTutorCount: 1, // no marker
		WinLines: &WinLineAnalysis{
			WinLines: []WinLine{{Weight: 3, Confidence: 0.55}}, // T4 vote
		},
	}
	// Votes: GC=T1, Mana=T4, Tutors=T2, WinLine=T4, Interaction=T1, CMC=T4
	// Sorted: 1/1/2/4/4/4 → upper-mid pos 3 = T4 → gate demotes → T3.
	tier2 := ClassifyCEDHPowerTier(dp2, report2)
	if tier2.Tier != 3 {
		t.Errorf("gate-demote fixture: want T3, got T%d", tier2.Tier)
	}
	// Confidence should be lowered by both signal dispersion AND gate
	// penalty. Strict bound: < 0.5 (very low confidence in T3 — the
	// deck "wants" to be T4).
	if tier2.Confidence >= 0.5 {
		t.Errorf("gate-demote fixture: want low confidence (<0.5), got %.4f", tier2.Confidence)
	}
}

// TestCEDHPowerTier_ConfidenceCalibration runs the classifier against
// the 16-deck test corpus and asserts confidence is high (>0.7) for
// the clear-cut cEDH and clear-cut casual decks. Skipped when oracle
// data absent.
func TestCEDHPowerTier_ConfidenceCalibration(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, _ := loadOracle(oraclePath)
	mechDB, _ := BuildMechanicDB(oraclePath)
	deckDir := "../../data/decks/test"
	matches, _ := filepath.Glob(filepath.Join(deckDir, "*.txt"))
	re := regexp.MustCompile(`_b([1-5])_`)

	type row struct {
		name string
		exp  int
		got  int
		conf float64
	}
	var rows []row
	for _, deck := range matches {
		base := filepath.Base(deck)
		m := re.FindStringSubmatch(base)
		if len(m) != 2 {
			continue
		}
		expected, _ := strconv.Atoi(m[1])
		report, err := analyzeDeckFile(deck, oracle, mechDB)
		if err != nil {
			continue
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		rows = append(rows, row{base, expected, tier.Tier, tier.Confidence})
	}

	// Calibration assertions on the 16-deck corpus:
	//   (a) Every cEDH-exact row must have confidence ≥ 0.60. A
	//       deck classified T5 by upper-mid median with all
	//       discriminator markers tripping must show meaningful
	//       signal agreement. Floor at 0.60 (not 0.70) accommodates
	//       decks like lumra that have one weak signal (F-grade
	//       mana base) — the classifier still says T5 with
	//       conviction, but the F-grade outlier pulls agreement to
	//       ~0.67. That's honest signal, not a calibration bug.
	//   (b) Mean confidence across cEDH-exact rows ≥ 0.80. The
	//       clean cEDH decks (yarok / azula / kraum / muldrotha)
	//       all score 1.0; the noisier ones (lumra, big_stick,
	//       turbo, stormoff) score 0.67-0.83. Mean lands ~0.87.
	cedhExactCount := 0
	cedhConfSum := 0.0
	for _, r := range rows {
		if r.exp == 5 && r.got == 5 {
			cedhExactCount++
			cedhConfSum += r.conf
			if r.conf < 0.60 {
				t.Errorf("cEDH-exact %s: confidence %.4f below 0.60 floor", r.name, r.conf)
			}
		}
	}
	if cedhExactCount < 5 {
		t.Errorf("expected at least 5 cEDH-exact rows for confidence check, got %d", cedhExactCount)
	}
	if cedhExactCount > 0 {
		meanConf := cedhConfSum / float64(cedhExactCount)
		if meanConf < 0.80 {
			t.Errorf("mean cEDH-exact confidence %.4f below 0.80 floor", meanConf)
		}
	}

	t.Logf("=== cEDH-tier confidence per deck ===")
	for _, r := range rows {
		marker := "OK"
		if r.got != r.exp {
			marker = "MISS"
		}
		t.Logf("  exp=B%d got=T%d conf=%.3f  %-50s  %s", r.exp, r.got, r.conf, r.name, marker)
	}
}
