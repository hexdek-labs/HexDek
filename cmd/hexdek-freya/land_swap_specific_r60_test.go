package main

import (
	"strings"
	"testing"
)

// makeLandSwapReport builds the minimal FreyaReport scaffolding
// computeLandSwapSuggestions needs: Stats non-nil + ColorDemand /
// ColorSupply maps + a Profiles slice the swap-out picker can scan.
func makeLandSwapReport(demand, supply map[string]int, lands []CardProfile) *FreyaReport {
	return &FreyaReport{
		Stats:       &DeckStatistics{},
		ColorDemand: demand,
		ColorSupply: supply,
		Profiles:    lands,
	}
}

// TestComputeLandSwapSuggestions_SpecificCardSwap pins the canonical
// output shape: "Recommend swap: <deck land> → <dual recommendation>
// (<undersupply> demand high, <oversupply> demand low)". This is the
// task spec's exemplar.
func TestComputeLandSwapSuggestions_SpecificCardSwap(t *testing.T) {
	// 3-color deck (W/R/G) with high G demand and low R demand.
	// Demand: G=15, W=5, R=2 — G is heavily under-supplied.
	// Supply: G=4,  W=4, R=8 — R is over-supplied.
	demand := map[string]int{"G": 15, "W": 5, "R": 2}
	supply := map[string]int{"G": 4, "W": 4, "R": 8}
	// Mishra's Factory: nonbasic, produces only R — the canonical
	// "stranded on wrong color" swap candidate. Bountiful Promenade
	// (G/W) is NOT in the deck, so the picker should recommend it.
	lands := []CardProfile{
		{Name: "Mishra's Factory", IsLand: true, LandColors: []string{"R"}},
		{Name: "Sunpetal Grove", IsLand: true, LandColors: []string{"W", "G"}},
	}
	dp := &DeckProfile{}
	report := makeLandSwapReport(demand, supply, lands)
	computeLandSwapSuggestions(dp, report)
	if len(dp.LandSwapSuggestions) == 0 {
		t.Fatal("expected at least one swap suggestion, got none")
	}
	got := dp.LandSwapSuggestions[0]
	for _, want := range []string{
		"Recommend swap:",
		"Mishra's Factory",
		"Bountiful Promenade",
		"G demand high",
		"R demand low",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("suggestion %q missing %q", got, want)
		}
	}
}

// TestPickLandSwapCandidate_StrandedSingleColorFirst pins the
// preference order: a mono-color nonbasic (Mishra's Factory) outranks
// a 2-color nonbasic that doesn't include the undersupplied color
// (Sulfurous Springs in a G-undersupplied / B-oversupplied deck).
// Stranded mono-color lands are the most actionable swaps.
func TestPickLandSwapCandidate_StrandedSingleColorFirst(t *testing.T) {
	lands := []CardProfile{
		{Name: "Sulfurous Springs", IsLand: true, LandColors: []string{"B", "R"}},
		{Name: "Mishra's Factory", IsLand: true, LandColors: []string{"R"}},
	}
	report := &FreyaReport{Profiles: lands}
	got := pickLandSwapCandidate(report, "R", "G")
	if got != "Mishra's Factory" {
		t.Errorf("expected mono-color stranded land first, got %q", got)
	}
}

// TestPickLandSwapCandidate_SkipsAlreadyPullingWeight pins that a
// dual producing BOTH the oversupplied AND undersupplied colors is
// NOT a swap candidate — it's already helping the undersupplied
// color. Sunpetal Grove (W/G) should be skipped when the imbalance
// is "G undersupplied, W oversupplied" because removing it costs
// you a G source too.
func TestPickLandSwapCandidate_SkipsAlreadyPullingWeight(t *testing.T) {
	lands := []CardProfile{
		{Name: "Sunpetal Grove", IsLand: true, LandColors: []string{"W", "G"}},
		{Name: "Plains", IsLand: true, LandColors: []string{"W"}},
	}
	report := &FreyaReport{Profiles: lands}
	got := pickLandSwapCandidate(report, "W", "G")
	if got == "Sunpetal Grove" {
		t.Errorf("Sunpetal Grove produces G — must not be a swap-out candidate, got %q", got)
	}
	if got != "Plains" {
		t.Errorf("expected Plains (mono-W stranded), got %q", got)
	}
}

// TestPickReplacementDual_PrefersHighestDemandSecondColor pins that
// the recommended dual's SECOND color is the deck's most-demanded
// in-deck color (excluding the undersupplied color itself). Sets up
// a B/R/G deck with G undersupplied — B has higher demand than R, so
// the recommendation should pair G with B (Undergrowth Stadium).
func TestPickReplacementDual_PrefersHighestDemandSecondColor(t *testing.T) {
	deckColors := map[string]bool{"B": true, "R": true, "G": true}
	demand := map[string]int{"B": 12, "R": 5, "G": 15}
	inDeck := map[string]bool{}
	got := pickReplacementDual("G", deckColors, demand, inDeck)
	if got != "Undergrowth Stadium" {
		t.Errorf("expected B/G dual (Undergrowth Stadium — B has higher peer demand than R), got %q", got)
	}
}

// TestPickReplacementDual_SkipsAlreadyInDeck pins that a dual the
// player already runs (Bountiful Promenade) is not re-recommended —
// Spire Garden (G/R) becomes the fallback since R is the next-best
// peer color.
func TestPickReplacementDual_SkipsAlreadyInDeck(t *testing.T) {
	deckColors := map[string]bool{"W": true, "R": true, "G": true}
	demand := map[string]int{"W": 12, "R": 6, "G": 15}
	inDeck := map[string]bool{"bountiful promenade": true}
	got := pickReplacementDual("G", deckColors, demand, inDeck)
	if got == "Bountiful Promenade" {
		t.Errorf("must skip duals already in the deck, got %q", got)
	}
	if got != "Spire Garden" {
		t.Errorf("expected Spire Garden (next-best peer = R), got %q", got)
	}
}

// TestPickReplacementDual_MonoColorReturnsEmpty pins that a single-
// color deck (only one color in identity) has NO eligible duals,
// signaling the caller to fall back to basic-land recommendation.
func TestPickReplacementDual_MonoColorReturnsEmpty(t *testing.T) {
	deckColors := map[string]bool{"G": true}
	demand := map[string]int{"G": 18}
	got := pickReplacementDual("G", deckColors, map[string]int{}, map[string]bool{})
	_ = demand
	if got != "" {
		t.Errorf("mono-color deck has no dual peers — expected empty, got %q", got)
	}
}

// TestComputeLandSwapSuggestions_BalancedDeckEmitsNothing pins that
// a deck where every color's supply tracks demand within the 5%
// tolerance produces no swap suggestions. Avoids spurious noise on
// well-tuned manabases.
func TestComputeLandSwapSuggestions_BalancedDeckEmitsNothing(t *testing.T) {
	demand := map[string]int{"W": 10, "U": 10, "G": 10}
	supply := map[string]int{"W": 10, "U": 10, "G": 10}
	dp := &DeckProfile{}
	computeLandSwapSuggestions(dp, makeLandSwapReport(demand, supply, nil))
	if len(dp.LandSwapSuggestions) != 0 {
		t.Errorf("balanced deck produced %d unwanted suggestions: %v",
			len(dp.LandSwapSuggestions), dp.LandSwapSuggestions)
	}
}

// TestComputeLandSwapSuggestions_FallbackWhenNoNonbasicCandidate
// pins the fallback path: if the deck's only oversupplied source is
// basics (which aren't in report.Profiles), the swap suggestion
// falls back to the legacy "Replace 1 Mountain with 1 Bountiful
// Promenade" format instead of being silently dropped.
func TestComputeLandSwapSuggestions_FallbackWhenNoNonbasicCandidate(t *testing.T) {
	demand := map[string]int{"G": 15, "W": 5, "R": 2}
	supply := map[string]int{"G": 4, "W": 4, "R": 8}
	// No nonbasic lands in the deck profile — all R supply is from
	// basic Mountains (which Freya doesn't put in report.Profiles).
	dp := &DeckProfile{}
	computeLandSwapSuggestions(dp, makeLandSwapReport(demand, supply, nil))
	if len(dp.LandSwapSuggestions) == 0 {
		t.Fatal("expected fallback suggestion, got none")
	}
	got := dp.LandSwapSuggestions[0]
	if !strings.Contains(got, "Mountain") || !strings.Contains(got, "Bountiful Promenade") {
		t.Errorf("fallback should reference Mountain → Bountiful Promenade, got %q", got)
	}
}

// TestColorPairKey_OrderIndependent pins the alphabetical canonical
// form so the dual-recommendation lookup works regardless of caller
// argument order.
func TestColorPairKey_OrderIndependent(t *testing.T) {
	if colorPairKey("G", "W") != colorPairKey("W", "G") {
		t.Errorf("colorPairKey not order-independent: %q vs %q",
			colorPairKey("G", "W"), colorPairKey("W", "G"))
	}
	if colorPairKey("G", "W") != "GW" {
		t.Errorf("expected canonical GW, got %q", colorPairKey("G", "W"))
	}
}
