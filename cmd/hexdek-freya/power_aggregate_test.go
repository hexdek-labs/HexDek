package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// makeAggregateTestReport stitches together the minimum FreyaReport
// shape ComputePowerTierAggregate needs: a Profile with bracket,
// archetype, and pre-populated CardPowerLevels.
func makeAggregateTestReport(bracket int, bracketLabel string, arch string, powers []int) *FreyaReport {
	pls := make([]CardPowerLevel, len(powers))
	for i, p := range powers {
		pls[i] = CardPowerLevel{
			Name:      fmt.Sprintf("c%d", i),
			Power:     p,
			PowerTier: PowerTierFor(p),
		}
	}
	return &FreyaReport{
		Profile: &DeckProfile{
			Bracket:          bracket,
			BracketLabel:     bracketLabel,
			PrimaryArchetype: arch,
			CardPowerLevels:  pls,
		},
	}
}

// TestComputePowerTierAggregate_Empty verifies the empty-input path
// returns a zero-value aggregate (not nil) so callers can safely
// inspect TierCounts / TierPercents without nil checks.
func TestComputePowerTierAggregate_Empty(t *testing.T) {
	agg := ComputePowerTierAggregate(nil)
	if agg == nil {
		t.Fatalf("empty input should still return non-nil aggregate")
	}
	if agg.DeckCount != 0 || agg.TotalCards != 0 {
		t.Errorf("empty aggregate should have zero counts, got Decks=%d Cards=%d",
			agg.DeckCount, agg.TotalCards)
	}
	for _, tier := range PowerTierOrder {
		if _, ok := agg.TierCounts[tier]; !ok {
			t.Errorf("TierCounts missing seeded key %q in empty aggregate", tier)
		}
	}
}

// TestComputePowerTierAggregate_TierCountsAndPercents verifies the
// overall tier histogram tallies + percent computations across a small
// hand-crafted set.
func TestComputePowerTierAggregate_TierCountsAndPercents(t *testing.T) {
	// Powers chosen to land deterministically: 80(S) 65(A) 50(B) 30(C) 10(D).
	reports := []*FreyaReport{
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{80, 65, 50, 30, 10}),
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{75, 60, 40, 25, 0}),
	}
	agg := ComputePowerTierAggregate(reports)

	wantCounts := map[string]int{"S": 2, "A": 2, "B": 2, "C": 2, "D": 2}
	for tier, want := range wantCounts {
		if got := agg.TierCounts[tier]; got != want {
			t.Errorf("tier %s count: want %d, got %d", tier, want, got)
		}
	}
	if agg.TotalCards != 10 {
		t.Errorf("total cards: want 10, got %d", agg.TotalCards)
	}
	if agg.DeckCount != 2 {
		t.Errorf("deck count: want 2, got %d", agg.DeckCount)
	}
	// Each tier should be 20% (2/10).
	for _, tier := range PowerTierOrder {
		if pct := agg.TierPercents[tier]; pct < 0.19 || pct > 0.21 {
			t.Errorf("tier %s percent: want ~0.20, got %.3f", tier, pct)
		}
	}
}

// TestComputePowerTierAggregate_MeanMedianMinMax pins the summary
// statistics across a deterministic input.
func TestComputePowerTierAggregate_MeanMedianMinMax(t *testing.T) {
	reports := []*FreyaReport{
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{10, 30, 50, 70, 90}),
	}
	agg := ComputePowerTierAggregate(reports)
	if agg.MinPower != 10 {
		t.Errorf("min: want 10, got %d", agg.MinPower)
	}
	if agg.MaxPower != 90 {
		t.Errorf("max: want 90, got %d", agg.MaxPower)
	}
	if agg.MedianPower != 50 {
		t.Errorf("median: want 50, got %d", agg.MedianPower)
	}
	if agg.MeanPower != 50.0 {
		t.Errorf("mean: want 50.0, got %.2f", agg.MeanPower)
	}
}

// TestComputePowerTierAggregate_ScoreHistogram_BinShape verifies the
// 21-bin 5-point histogram (0-4, 5-9, ..., 95-100) has the correct
// boundaries and is exclusive of out-of-range counts.
func TestComputePowerTierAggregate_ScoreHistogram_BinShape(t *testing.T) {
	reports := []*FreyaReport{
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{0, 4, 5, 99, 100}),
	}
	agg := ComputePowerTierAggregate(reports)
	if len(agg.ScoreHistogram) != 21 {
		t.Fatalf("histogram should have 21 bins, got %d", len(agg.ScoreHistogram))
	}
	// Bin 0 = [0, 4]: 0 and 4 both land here. Bin 1 = [5, 9]: 5 lands.
	// Bin 19 = [95, 99]: 99. Bin 20 = [95, 100] (top bin extends to 100).
	if agg.ScoreHistogram[0].Count != 2 {
		t.Errorf("bin [0,4]: want 2 (0+4), got %d", agg.ScoreHistogram[0].Count)
	}
	if agg.ScoreHistogram[1].Count != 1 {
		t.Errorf("bin [5,9]: want 1 (5), got %d", agg.ScoreHistogram[1].Count)
	}
	// 100 must land in the top bin (RangeEnd extended to 100).
	if agg.ScoreHistogram[20].Count != 1 {
		t.Errorf("bin [95,100]: want 1 (the 100), got %d", agg.ScoreHistogram[20].Count)
	}
	if agg.ScoreHistogram[20].RangeEnd != 100 {
		t.Errorf("top bin RangeEnd: want 100 (extended), got %d", agg.ScoreHistogram[20].RangeEnd)
	}
}

// TestComputePowerTierAggregate_ByBracketGroupsCorrectly verifies the
// per-bracket breakdown groups decks by their Bracket field and
// surfaces them sorted ascending.
func TestComputePowerTierAggregate_ByBracketGroupsCorrectly(t *testing.T) {
	reports := []*FreyaReport{
		makeAggregateTestReport(5, "cEDH", "Combo", []int{90, 80}),
		makeAggregateTestReport(5, "cEDH", "Combo", []int{85, 75}),
		makeAggregateTestReport(2, "Core", "Tribal", []int{40, 30}),
	}
	agg := ComputePowerTierAggregate(reports)
	if len(agg.ByBracket) != 2 {
		t.Fatalf("want 2 bracket rows, got %d", len(agg.ByBracket))
	}
	// Ascending bracket order.
	if agg.ByBracket[0].Bracket != 2 {
		t.Errorf("first bracket row should be B2, got B%d", agg.ByBracket[0].Bracket)
	}
	if agg.ByBracket[1].Bracket != 5 {
		t.Errorf("second bracket row should be B5, got B%d", agg.ByBracket[1].Bracket)
	}
	if agg.ByBracket[1].DeckCount != 2 {
		t.Errorf("B5 deck count: want 2, got %d", agg.ByBracket[1].DeckCount)
	}
	if agg.ByBracket[1].TierCounts["S"] != 4 {
		t.Errorf("B5 S count: want 4 (all 4 cards 75+), got %d", agg.ByBracket[1].TierCounts["S"])
	}
}

// TestComputePowerTierAggregate_ByArchetypeSortsByDeckCount verifies
// archetypes are sorted by deck count descending so the most-
// represented archetypes lead the calibration table.
func TestComputePowerTierAggregate_ByArchetypeSortsByDeckCount(t *testing.T) {
	reports := []*FreyaReport{
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{50}),
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{55}),
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{60}),
		makeAggregateTestReport(3, "Upgraded", "Stax", []int{45}),
		makeAggregateTestReport(3, "Upgraded", "Tribal", []int{40}),
		makeAggregateTestReport(3, "Upgraded", "Tribal", []int{50}),
	}
	agg := ComputePowerTierAggregate(reports)
	if len(agg.ByArchetype) != 3 {
		t.Fatalf("want 3 archetype rows, got %d", len(agg.ByArchetype))
	}
	if agg.ByArchetype[0].Archetype != "Combo" {
		t.Errorf("most-represented archetype should be Combo (3 decks), got %q",
			agg.ByArchetype[0].Archetype)
	}
	if agg.ByArchetype[0].DeckCount != 3 {
		t.Errorf("Combo deck count: want 3, got %d", agg.ByArchetype[0].DeckCount)
	}
	if agg.ByArchetype[1].Archetype != "Tribal" {
		t.Errorf("second-most archetype should be Tribal (2 decks), got %q",
			agg.ByArchetype[1].Archetype)
	}
	if agg.ByArchetype[2].Archetype != "Stax" {
		t.Errorf("least-represented archetype should be Stax (1 deck), got %q",
			agg.ByArchetype[2].Archetype)
	}
}

// TestComputePowerTierAggregate_SkipsEmptyProfiles verifies that
// reports without a Profile or without CardPowerLevels are skipped
// rather than blowing up — defensive against pipeline failures.
func TestComputePowerTierAggregate_SkipsEmptyProfiles(t *testing.T) {
	reports := []*FreyaReport{
		nil, // nil report
		{},  // no Profile
		{Profile: &DeckProfile{}},                                     // Profile but no CardPowerLevels
		makeAggregateTestReport(3, "Upgraded", "Combo", []int{50, 75}), // valid
	}
	agg := ComputePowerTierAggregate(reports)
	if agg.DeckCount != 1 {
		t.Errorf("only 1 deck should be counted (the valid one), got %d", agg.DeckCount)
	}
	if agg.TotalCards != 2 {
		t.Errorf("only 2 cards should be counted, got %d", agg.TotalCards)
	}
}

// TestPrintPowerTierAggregate_SectionsPresent verifies the text report
// includes all four sections (header, overall mix, per-bracket,
// per-archetype, histogram).
func TestPrintPowerTierAggregate_SectionsPresent(t *testing.T) {
	reports := []*FreyaReport{
		makeAggregateTestReport(5, "cEDH", "Combo", []int{90, 80, 70, 60, 50, 40, 30, 20, 10}),
		makeAggregateTestReport(2, "Core", "Tribal", []int{50, 40, 30, 20}),
	}
	agg := ComputePowerTierAggregate(reports)
	var buf bytes.Buffer
	PrintPowerTierAggregate(&buf, agg)
	out := buf.String()

	sections := []string{
		"POWER-TIER DISTRIBUTION",
		"Mean power:",
		"Overall tier mix:",
		"By bracket:",
		"By primary archetype:",
		"Score histogram (5-point bins):",
	}
	for _, s := range sections {
		if !strings.Contains(out, s) {
			t.Errorf("aggregate output missing section %q\n--- output ---\n%s", s, out)
		}
	}
	// All 5 tier letters should appear in the overall mix line.
	for _, t2 := range PowerTierOrder {
		if !strings.Contains(out, t2+":") {
			t.Errorf("tier %q missing from overall-mix section", t2)
		}
	}
}

// TestPrintPowerTierAggregate_EmptyIsNoOp verifies empty input
// produces no output (rather than rendering a header for an empty
// aggregate, which would confuse the multi-deck summary).
func TestPrintPowerTierAggregate_EmptyIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	PrintPowerTierAggregate(&buf, ComputePowerTierAggregate(nil))
	if buf.Len() != 0 {
		t.Errorf("empty aggregate should render nothing, got %d bytes:\n%s", buf.Len(), buf.String())
	}
}
