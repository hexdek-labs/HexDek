package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPodBalance_BalancedPod_AllSameTier verifies that 4 decks at
// the same tier produce spread=0, balance="balanced", and verdict
// mentions "Balanced".
func TestPodBalance_BalancedPod_AllSameTier(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "deck_a", Tier: 3, Confidence: 0.85, Label: "Upgraded Precon"},
		{DeckName: "deck_b", Tier: 3, Confidence: 0.80, Label: "Upgraded Precon"},
		{DeckName: "deck_c", Tier: 3, Confidence: 0.90, Label: "Upgraded Precon"},
		{DeckName: "deck_d", Tier: 3, Confidence: 0.75, Label: "Upgraded Precon"},
	}
	a := AssessPodBalance(pod)
	if a.TierSpread != 0 {
		t.Errorf("spread: want 0, got %d", a.TierSpread)
	}
	if a.Balance != "balanced" {
		t.Errorf("balance: want balanced, got %q", a.Balance)
	}
	if a.DominantTier != 3 {
		t.Errorf("dominant: want 3, got %d", a.DominantTier)
	}
	if a.AvgTier != 3.0 {
		t.Errorf("avg: want 3.0, got %.4f", a.AvgTier)
	}
	if len(a.Outliers) != 0 {
		t.Errorf("outliers: want 0, got %v", a.Outliers)
	}
	if !strings.Contains(a.Verdict, "Balanced") {
		t.Errorf("verdict missing 'Balanced': %q", a.Verdict)
	}
}

// TestPodBalance_BalancedPod_WithinOneTier verifies the user's
// spec'd "good pod" criterion: spread of 1 (e.g. B3/B3/B4/B4) is
// still "balanced" per the "within 1 tier of each other" rule.
func TestPodBalance_BalancedPod_WithinOneTier(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "precon_3a", Tier: 3, Confidence: 0.85},
		{DeckName: "precon_3b", Tier: 3, Confidence: 0.80},
		{DeckName: "high_pwr_4a", Tier: 4, Confidence: 0.88},
		{DeckName: "high_pwr_4b", Tier: 4, Confidence: 0.92},
	}
	a := AssessPodBalance(pod)
	if a.TierSpread != 1 {
		t.Errorf("spread: want 1, got %d", a.TierSpread)
	}
	if a.Balance != "balanced" {
		t.Errorf("balance for spread=1: want balanced, got %q", a.Balance)
	}
	if a.AvgTier != 3.5 {
		t.Errorf("avg: want 3.5, got %.4f", a.AvgTier)
	}
	// Dominant tier on a 2-2 split: conservative tie-break picks
	// the LOWER tier (T3 here).
	if a.DominantTier != 3 {
		t.Errorf("dominant on 2-2 split: want 3 (lower-tier tie-break), got %d", a.DominantTier)
	}
	if len(a.Outliers) != 0 {
		t.Errorf("outliers for spread=1: want 0, got %v", a.Outliers)
	}
}

// TestPodBalance_MildImbalance_SpreadTwo verifies spread of 2 →
// "mild_imbalance" — e.g. B3/B3/B4/B5.
func TestPodBalance_MildImbalance_SpreadTwo(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "precon_3", Tier: 3},
		{DeckName: "precon_3b", Tier: 3},
		{DeckName: "high_pwr_4", Tier: 4},
		{DeckName: "cedh_5", Tier: 5},
	}
	a := AssessPodBalance(pod)
	if a.TierSpread != 2 {
		t.Errorf("spread: want 2, got %d", a.TierSpread)
	}
	if a.Balance != "mild_imbalance" {
		t.Errorf("balance: want mild_imbalance, got %q", a.Balance)
	}
	// DominantTier T3 (2 votes); cedh_5 at T5 is 2 tiers off → outlier
	if a.DominantTier != 3 {
		t.Errorf("dominant: want 3, got %d", a.DominantTier)
	}
	if len(a.Outliers) != 1 || a.Outliers[0] != "cedh_5" {
		t.Errorf("outliers: want [cedh_5], got %v", a.Outliers)
	}
	if !strings.Contains(a.Verdict, "Mildly imbalanced") {
		t.Errorf("verdict missing label: %q", a.Verdict)
	}
}

// TestPodBalance_Imbalanced_SpreadThree verifies spread of 3 →
// "imbalanced" — e.g. B2/B3/B4/B5.
func TestPodBalance_Imbalanced_SpreadThree(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "casual_2", Tier: 2},
		{DeckName: "precon_3", Tier: 3},
		{DeckName: "high_pwr_4", Tier: 4},
		{DeckName: "cedh_5", Tier: 5},
	}
	a := AssessPodBalance(pod)
	if a.TierSpread != 3 {
		t.Errorf("spread: want 3, got %d", a.TierSpread)
	}
	if a.Balance != "imbalanced" {
		t.Errorf("balance: want imbalanced, got %q", a.Balance)
	}
	if a.AvgTier != 3.5 {
		t.Errorf("avg: want 3.5, got %.4f", a.AvgTier)
	}
}

// TestPodBalance_Lopsided_SpreadFour verifies the user's spec'd
// "bad pod" criterion: spread of 4 (B1-B5 spread) → "lopsided".
func TestPodBalance_Lopsided_SpreadFour(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "casual_1a", Tier: 1},
		{DeckName: "casual_1b", Tier: 1},
		{DeckName: "casual_1c", Tier: 1},
		{DeckName: "cedh_5", Tier: 5},
	}
	a := AssessPodBalance(pod)
	if a.TierSpread != 4 {
		t.Errorf("spread: want 4, got %d", a.TierSpread)
	}
	if a.Balance != "lopsided" {
		t.Errorf("balance: want lopsided, got %q", a.Balance)
	}
	if a.DominantTier != 1 {
		t.Errorf("dominant: want 1, got %d", a.DominantTier)
	}
	// cedh_5 is 4 tiers off T1 → outlier
	if len(a.Outliers) != 1 || a.Outliers[0] != "cedh_5" {
		t.Errorf("outliers: want [cedh_5], got %v", a.Outliers)
	}
	if !strings.Contains(a.Verdict, "Lopsided") {
		t.Errorf("verdict missing 'Lopsided': %q", a.Verdict)
	}
}

// TestPodBalance_DominantTierTieBreak verifies the conservative
// lower-tier tie-break on 2-2 splits at non-adjacent tiers.
func TestPodBalance_DominantTierTieBreak(t *testing.T) {
	// 2 at T2, 2 at T5 — tie. Conservative picks T2 (the lower).
	pod := []PodDeckTier{
		{DeckName: "casual_a", Tier: 2},
		{DeckName: "casual_b", Tier: 2},
		{DeckName: "cedh_a", Tier: 5},
		{DeckName: "cedh_b", Tier: 5},
	}
	a := AssessPodBalance(pod)
	if a.DominantTier != 2 {
		t.Errorf("tie-break: want lower-tier 2, got %d", a.DominantTier)
	}
	if a.TierSpread != 3 {
		t.Errorf("spread: want 3, got %d", a.TierSpread)
	}
	// Both cEDH decks are 3 tiers from T2 → outliers
	if len(a.Outliers) != 2 {
		t.Errorf("outliers: want 2, got %d: %v", len(a.Outliers), a.Outliers)
	}
}

// TestPodBalance_AvgConfidence verifies confidence aggregation.
func TestPodBalance_AvgConfidence(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "a", Tier: 3, Confidence: 0.80},
		{DeckName: "b", Tier: 3, Confidence: 0.60},
		{DeckName: "c", Tier: 4, Confidence: 0.90},
		{DeckName: "d", Tier: 4, Confidence: 1.00},
	}
	a := AssessPodBalance(pod)
	want := (0.80 + 0.60 + 0.90 + 1.00) / 4
	if a.AvgConfidence < want-1e-9 || a.AvgConfidence > want+1e-9 {
		t.Errorf("avg confidence: want %.4f, got %.4f", want, a.AvgConfidence)
	}
}

// TestPodBalance_NoModeNoDominant verifies that a pod with no tier
// repeated (all 4 different) has DominantTier=0 and uses the rounded
// mean as the outlier reference.
func TestPodBalance_NoModeNoDominant(t *testing.T) {
	pod := []PodDeckTier{
		{DeckName: "a", Tier: 1},
		{DeckName: "b", Tier: 3},
		{DeckName: "c", Tier: 4},
		{DeckName: "d", Tier: 5},
	}
	a := AssessPodBalance(pod)
	if a.DominantTier != 0 {
		t.Errorf("no-mode pod: want DominantTier=0, got %d", a.DominantTier)
	}
	// avg = 3.25 → rounded ref tier = 3. T1 is 2 off (outlier); T5 is 2 off (outlier).
	if len(a.Outliers) != 2 {
		t.Errorf("no-mode outliers: want 2, got %d: %v", len(a.Outliers), a.Outliers)
	}
}

// TestPodBalance_EmptyAndSingleton verifies the under-2-deck guard.
func TestPodBalance_EmptyAndSingleton(t *testing.T) {
	for _, n := range []int{0, 1} {
		pod := make([]PodDeckTier, n)
		for i := range pod {
			pod[i] = PodDeckTier{DeckName: "x", Tier: 3}
		}
		a := AssessPodBalance(pod)
		if a == nil {
			t.Fatalf("n=%d: nil assessment", n)
		}
		if a.Balance != "empty" {
			t.Errorf("n=%d: want Balance=empty, got %q", n, a.Balance)
		}
		if !strings.Contains(a.Verdict, "too small") {
			t.Errorf("n=%d: verdict missing 'too small': %q", n, a.Verdict)
		}
	}
}

// TestPodBalance_BalanceBandTable pins balanceBandForSpread at every
// spread value, including out-of-range >4 defensively mapping to
// "lopsided" (already max-band).
func TestPodBalance_BalanceBandTable(t *testing.T) {
	cases := map[int]string{
		0: "balanced",
		1: "balanced",
		2: "mild_imbalance",
		3: "imbalanced",
		4: "lopsided",
		5: "lopsided", // defensive
	}
	for spread, want := range cases {
		got := balanceBandForSpread(spread)
		if got != want {
			t.Errorf("spread=%d: want %q, got %q", spread, want, got)
		}
	}
}

// TestPodBalance_VerdictSortedRender verifies that the verdict lists
// tiers in SORTED ascending order regardless of input order.
func TestPodBalance_VerdictSortedRender(t *testing.T) {
	// Pod with tiers in reverse order: T5, T4, T3, T3 — should
	// render as "T3 / T3 / T4 / T5" in the verdict.
	pod := []PodDeckTier{
		{DeckName: "d", Tier: 5},
		{DeckName: "c", Tier: 4},
		{DeckName: "b", Tier: 3},
		{DeckName: "a", Tier: 3},
	}
	a := AssessPodBalance(pod)
	if !strings.Contains(a.Verdict, "T3 / T3 / T4 / T5") {
		t.Errorf("verdict not sorted-ascending: %q", a.Verdict)
	}
}

// TestPodBalance_EndToEnd_RealDecks runs ClassifyCEDHPowerTier on 4
// real curated decks of mixed tiers and verifies the pod assessment
// surfaces the imbalance correctly. Skipped when oracle data absent.
func TestPodBalance_EndToEnd_RealDecks(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, err := loadOracle(oraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(oraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}

	// Mix: 1 cEDH B5 + 2 B4 nightmares + 1 B2 precon. Expected:
	// imbalanced or lopsided pod (cEDH vs precon at the same table).
	deckPaths := []string{
		"../../data/decks/test/cedh_blink_b5_yarok.txt",
		"../../data/decks/test/facedown_nightmare_b4_yarus.txt",
		"../../data/decks/test/modal_nightmare_b4_riku.txt",
		"../../data/decks/test/disguise_precon_b2_kaust.txt",
	}

	var pod []PodDeckTier
	for _, p := range deckPaths {
		report, err := analyzeDeckFile(p, oracle, mechDB)
		if err != nil {
			t.Fatalf("analyze %s: %v", p, err)
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		pod = append(pod, PodDeckTier{
			DeckName:   filepath.Base(p),
			Tier:       tier.Tier,
			Confidence: tier.Confidence,
			Label:      tier.Label,
		})
	}

	a := AssessPodBalance(pod)
	t.Logf("Mixed pod assessment: %s", a.Verdict)
	for _, d := range a.Decks {
		t.Logf("  T%d conf=%.3f  %s", d.Tier, d.Confidence, d.DeckName)
	}
	// Spread should be ≥ 2 (cEDH vs precon at the same table).
	if a.TierSpread < 2 {
		t.Errorf("mixed pod: want spread ≥ 2, got %d (tiers: %v)", a.TierSpread, podTierList(a))
	}
	if a.Balance == "balanced" {
		t.Errorf("mixed pod: want non-balanced, got %q", a.Balance)
	}
	// At least one outlier expected (the kaust precon should be 2+
	// tiers from the cEDH yarok / B4 nightmare cluster).
	if len(a.Outliers) == 0 {
		t.Errorf("mixed pod: want at least 1 outlier, got 0")
	}
}

// TestPodBalance_EndToEnd_BalancedPrecons runs the classifier on 4
// precons and verifies they form a balanced pod (all should land at
// T1-T3 per the PR #715 precon verification).
func TestPodBalance_EndToEnd_BalancedPrecons(t *testing.T) {
	oraclePath := "../../data/rules/oracle-cards.json"
	if _, err := os.Stat(oraclePath); err != nil {
		t.Skipf("oracle data not available")
	}
	oracle, err := loadOracle(oraclePath)
	if err != nil {
		t.Fatalf("load oracle: %v", err)
	}
	mechDB, err := BuildMechanicDB(oraclePath)
	if err != nil {
		t.Fatalf("build mechanic db: %v", err)
	}

	deckPaths := []string{
		"../../data/decks/wizards/animated_army_bloomburrow_commander_precon_decklist.txt",
		"../../data/decks/wizards/cabaretti_cacophony_streets_of_new_capenna_commander_precon_decklist.txt",
		"../../data/decks/wizards/bedecked_brokers_streets_of_new_capenna_commander_precon_decklist.txt",
		"../../data/decks/wizards/deadly_disguise_murders_at_karlov_manor_commander_precon_decklist.txt",
	}

	var pod []PodDeckTier
	for _, p := range deckPaths {
		report, err := analyzeDeckFile(p, oracle, mechDB)
		if err != nil {
			t.Fatalf("analyze %s: %v", p, err)
		}
		dp := BuildDeckProfile(report, oracle)
		tier := ClassifyCEDHPowerTier(dp, report)
		pod = append(pod, PodDeckTier{
			DeckName:   filepath.Base(p),
			Tier:       tier.Tier,
			Confidence: tier.Confidence,
			Label:      tier.Label,
		})
	}

	a := AssessPodBalance(pod)
	t.Logf("All-precon pod assessment: %s", a.Verdict)
	for _, d := range a.Decks {
		t.Logf("  T%d conf=%.3f  %s", d.Tier, d.Confidence, d.DeckName)
	}
	// 4 precons should never spread > 2 (all are casual/precon
	// per the PR #715 verification — none reach T4+).
	if a.TierSpread > 2 {
		t.Errorf("all-precon pod: want spread ≤ 2, got %d (tiers: %v)", a.TierSpread, podTierList(a))
	}
}

func podTierList(a *PodBalanceAssessment) []int {
	out := make([]int, len(a.Decks))
	for i, d := range a.Decks {
		out[i] = d.Tier
	}
	return out
}
