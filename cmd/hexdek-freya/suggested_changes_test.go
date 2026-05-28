package main

import (
	"strings"
	"testing"
)

// TestSuggestedChanges_NilInputs verifies the defensive zero-result
// path: nil dp or nil report returns a non-nil zero struct with the
// "insufficient data" summary.
func TestSuggestedChanges_NilInputs(t *testing.T) {
	got := BuildSuggestedChanges(nil, nil)
	if got == nil {
		t.Fatal("nil input: want non-nil zero result, got nil")
	}
	if got.TotalChanges != 0 {
		t.Errorf("nil input: want TotalChanges=0, got %d", got.TotalChanges)
	}
	if !strings.Contains(got.Summary, "Insufficient") {
		t.Errorf("nil input: want 'Insufficient' in Summary, got %q", got.Summary)
	}

	got2 := BuildSuggestedChanges(&DeckProfile{}, nil)
	if got2 == nil || got2.TotalChanges != 0 {
		t.Errorf("nil report: want zero result, got %+v", got2)
	}
}

// TestSuggestedChanges_ControlDeckWantsMoreCounters verifies the
// user's spec'd scenario: a Control deck under-spec'd on
// interaction should get specific counterspell+removal adds. Both
// the gap count and the specific card name should appear in the
// recommendation.
func TestSuggestedChanges_ControlDeckWantsMoreCounters(t *testing.T) {
	// Control bracket-3 deck with 4 interaction (under the
	// Control-archetype target of 12). UB color identity so Swords
	// to Plowshares (W) is filtered out and Counterspell (U) is in.
	dp := &DeckProfile{
		PrimaryArchetype: "Control",
		Bracket:          3,
		ColorIdentity:    []string{"U", "B"},
		DrawCount:        9,  // hits the bracket-3 target of 7
		RampCount:        9,  // hits the bracket-3 target of 8
		ManaBaseGrade:    "B",
	}
	report := &FreyaReport{
		RemovalCount: 2,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 2,
				RoleBoardWipe:    1, // has wipes, no wipe-add needed
			},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	if got == nil {
		t.Fatal("nil result")
	}
	// Interaction gap = 12 (Control target) - 4 (RemovalCount + Counterspells) = 8
	// Should produce up to 8 add candidates (limited by color-
	// filtered staples).
	if len(got.Adds) == 0 {
		t.Fatal("want at least 1 add (interaction gap), got 0")
	}
	// Counterspell (U) must be in the adds — it's the canonical
	// U-color staple and the deck is UB.
	foundCounterspell := false
	for _, a := range got.Adds {
		if a.CardName == "Counterspell" {
			foundCounterspell = true
			if a.Category != "interaction" {
				t.Errorf("Counterspell: want category 'interaction', got %q", a.Category)
			}
			if a.Priority != 8 {
				t.Errorf("Counterspell: want priority 8, got %d", a.Priority)
			}
			if !strings.Contains(a.Reason, "Control") {
				t.Errorf("Counterspell reason missing 'Control' archetype callout: %q", a.Reason)
			}
			if !strings.Contains(a.Reason, "12") {
				t.Errorf("Counterspell reason missing target count '12': %q", a.Reason)
			}
		}
	}
	if !foundCounterspell {
		var addCardNames []string
		for _, a := range got.Adds {
			addCardNames = append(addCardNames, a.CardName)
		}
		t.Errorf("UB Control deck: want Counterspell in adds, got %v", addCardNames)
	}
	// Swords to Plowshares is W-only, should NOT be in a UB deck's
	// recommendations.
	for _, a := range got.Adds {
		if a.CardName == "Swords to Plowshares" {
			t.Errorf("UB deck got W-only Swords to Plowshares — color filter failed")
		}
	}
	// Summary should mention the highest-priority recommendation.
	if !strings.Contains(got.Summary, "Counterspell") && !strings.Contains(got.Summary, "Highest priority") {
		t.Errorf("Summary missing highest-priority callout: %q", got.Summary)
	}
}

// TestSuggestedChanges_ManabaseSwapTaplandForUpgrade verifies the
// mana-base swap path: a C-grade mana base with named tap-lands
// produces specific swap recommendations for known tap-land →
// upgrade pairs.
func TestSuggestedChanges_ManabaseSwapTaplandForUpgrade(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		ColorIdentity:    []string{"W", "U"},
		ManaBaseGrade:    "C",
		TaplandCount:     6,
		LandCount:        37,
		DrawCount:        7,
		RampCount:        8,
	}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Tranquil Cove", TypeLine: "Land", IsLand: true},
			{Name: "Azorius Guildgate", TypeLine: "Land", IsLand: true},
			// Non-tapland — should NOT generate a swap
			{Name: "Hallowed Fountain", TypeLine: "Land", IsLand: true},
		},
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{RoleBoardWipe: 1},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	if got == nil {
		t.Fatal("nil result")
	}
	// Should produce swaps for Tranquil Cove and Azorius Guildgate.
	// Should NOT swap a tapland whose upgrade is already owned —
	// Azorius Guildgate's upgrade in taplandUpgradeMap is
	// "Hallowed Fountain", which IS in the deck → skip.
	wantSwaps := map[string]string{
		"Tranquil Cove": "Bountiful Promenade",
	}
	for _, sw := range got.Swaps {
		want, ok := wantSwaps[sw.CutCardName]
		if !ok {
			continue // unrelated swap (none expected)
		}
		if sw.AddCardName != want {
			t.Errorf("swap for %s: want %q, got %q", sw.CutCardName, want, sw.AddCardName)
		}
		if sw.Category != "manabase" {
			t.Errorf("swap %s: want category manabase, got %q", sw.CutCardName, sw.Category)
		}
		if !strings.Contains(sw.Reason, "C-grade") {
			t.Errorf("swap reason missing grade callout: %q", sw.Reason)
		}
		delete(wantSwaps, sw.CutCardName)
	}
	if len(wantSwaps) > 0 {
		var swapsList []string
		for _, sw := range got.Swaps {
			swapsList = append(swapsList, sw.CutCardName+"->"+sw.AddCardName)
		}
		t.Errorf("missing expected swaps: %v (got swaps: %v)", wantSwaps, swapsList)
	}
	// Azorius Guildgate should NOT generate a swap (its upgrade
	// Hallowed Fountain is already owned).
	for _, sw := range got.Swaps {
		if sw.CutCardName == "Azorius Guildgate" {
			t.Errorf("Azorius Guildgate should not swap (Hallowed Fountain already owned)")
		}
	}
}

// TestSuggestedChanges_NoSwapsForGoodManaBase verifies the
// inverse: a B-grade or better mana base produces zero swap
// suggestions even with named tap-lands in the deck.
func TestSuggestedChanges_NoSwapsForGoodManaBase(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		ColorIdentity:    []string{"W", "U"},
		ManaBaseGrade:    "B",
		LandCount:        37,
		DrawCount:        7,
		RampCount:        8,
	}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Tranquil Cove", TypeLine: "Land", IsLand: true},
		},
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{RoleBoardWipe: 1},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	if len(got.Swaps) != 0 {
		t.Errorf("B-grade mana base: want 0 swaps, got %d", len(got.Swaps))
	}
}

// TestSuggestedChanges_CutsFromExistingCuttableCards verifies that
// Cuts come from the existing dp.CuttableCards list (Freya's
// synergy-tier-cuttable analysis) and that D-tier cards rank above
// C-tier in priority.
func TestSuggestedChanges_CutsFromExistingCuttableCards(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		ColorIdentity:    []string{"G"},
		LandCount:        37,
		DrawCount:        7,
		RampCount:        8,
		ManaBaseGrade:    "B",
		CuttableCards: []CardQuality{
			{Name: "Mid Filler", PowerTier: "C", Reason: "filler slot"},
			{Name: "Dead Slot", PowerTier: "D", Reason: "off-archetype dead slot"},
		},
	}
	report := &FreyaReport{
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{RoleBoardWipe: 1},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	if len(got.Cuts) != 2 {
		t.Fatalf("want 2 cuts, got %d (%+v)", len(got.Cuts), got.Cuts)
	}
	// Dead Slot (D-tier) should rank above Mid Filler (C-tier).
	if got.Cuts[0].CardName != "Dead Slot" {
		t.Errorf("expected D-tier 'Dead Slot' ranked first, got %q (full order: %v)",
			got.Cuts[0].CardName, cutNames(got.Cuts))
	}
	if got.Cuts[0].Priority != 5 {
		t.Errorf("D-tier cut priority: want 5, got %d", got.Cuts[0].Priority)
	}
	if got.Cuts[1].Priority != 4 {
		t.Errorf("C-tier cut priority: want 4, got %d", got.Cuts[1].Priority)
	}
	if !strings.Contains(got.Cuts[0].Reason, "off-archetype") {
		t.Errorf("D-tier cut reason missing original detail: %q", got.Cuts[0].Reason)
	}
}

// TestSuggestedChanges_NoWipes_AddsAWipe verifies the wipe-gap
// detection: a deck with zero RoleBoardWipe should get a wipe add
// (color-filtered).
func TestSuggestedChanges_NoWipes_AddsAWipe(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          4,
		ColorIdentity:    []string{"W", "B"},
		DrawCount:        10,
		RampCount:        10,
		ManaBaseGrade:    "B",
	}
	report := &FreyaReport{
		RemovalCount: 12, // hits the bracket-4 interaction target
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 0,
				RoleBoardWipe:    0, // zero — should trigger a wipe add
			},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	foundWipe := false
	for _, a := range got.Adds {
		if a.Category == "wipes" {
			foundWipe = true
			if a.Priority != 8 { // bracket >= 4 lifts priority to 8
				t.Errorf("wipe add: want priority 8 at bracket 4, got %d", a.Priority)
			}
			// Must be a WB-allowed wipe (Toxic Deluge or Damnation
			// or Wrath of God).
			validWipes := map[string]bool{
				"Toxic Deluge":  true,
				"Damnation":     true,
				"Wrath of God":  true,
				"Farewell":      true,
			}
			if !validWipes[a.CardName] {
				t.Errorf("wipe add: got %q, expected a WB-allowed wipe", a.CardName)
			}
		}
	}
	if !foundWipe {
		t.Errorf("WB deck with 0 wipes: want a wipe add, got %v", addCardNames(got.Adds))
	}
}

// TestSuggestedChanges_OwnedCardsSkipped verifies the owned-card
// filter — if a deck already has Sol Ring, the ramp-gap fill
// should pick the next-highest staple (Mana Crypt / Arcane Signet)
// instead of re-recommending Sol Ring.
func TestSuggestedChanges_OwnedCardsSkipped(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		ColorIdentity:    []string{"G"},
		DrawCount:        7,
		RampCount:        3, // far under target 8
		LandCount:        37,
		ManaBaseGrade:    "B",
	}
	report := &FreyaReport{
		Profiles: []CardProfile{
			{Name: "Sol Ring", TypeLine: "Artifact"},
		},
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{RoleBoardWipe: 1},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	for _, a := range got.Adds {
		if a.CardName == "Sol Ring" {
			t.Errorf("owned-card filter failed: Sol Ring already in deck but suggested as add")
		}
	}
	// Should still get OTHER ramp suggestions (Mana Crypt is
	// colorless, no color filter blocks it).
	foundRamp := false
	for _, a := range got.Adds {
		if a.Category == "ramp" {
			foundRamp = true
			break
		}
	}
	if !foundRamp {
		t.Errorf("ramp gap not filled despite ramp count well under target: %v", addCardNames(got.Adds))
	}
}

// TestSuggestedChanges_WellTunedDeckEmpty verifies the
// "no changes" path: a deck that hits all bracket-scaled targets
// and has good mana base returns a small, possibly empty change
// set.
func TestSuggestedChanges_WellTunedDeckEmpty(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Midrange",
		Bracket:          3,
		ColorIdentity:    []string{"G"},
		DrawCount:        9,  // > target 7
		RampCount:        10, // > target 8
		LandCount:        37,
		ManaBaseGrade:    "A",
	}
	report := &FreyaReport{
		RemovalCount: 9, // > target 8
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: 0,
				RoleBoardWipe:    1, // present
			},
		},
	}
	got := BuildSuggestedChanges(dp, report)
	if len(got.Adds) != 0 {
		t.Errorf("well-tuned deck: want 0 adds, got %d (%v)", len(got.Adds), addCardNames(got.Adds))
	}
	if len(got.Swaps) != 0 {
		t.Errorf("well-tuned deck: want 0 swaps, got %d", len(got.Swaps))
	}
	if !strings.Contains(got.Summary, "well-tuned") {
		t.Errorf("well-tuned summary: %q", got.Summary)
	}
}

// TestSuggestedChanges_SummaryShapeAndPrioritySort verifies the
// summary shape + that adds/cuts/swaps are sorted descending by
// priority.
func TestSuggestedChanges_SummaryShapeAndPrioritySort(t *testing.T) {
	dp := &DeckProfile{
		PrimaryArchetype: "Combo",
		Bracket:          4,
		ColorIdentity:    []string{"U", "B"},
		DrawCount:        4, // gap
		RampCount:        5, // gap
		LandCount:        37,
		ManaBaseGrade:    "D", // critical swap priority
		CuttableCards: []CardQuality{
			{Name: "Mid Filler", PowerTier: "C", Reason: "filler"},
			{Name: "Dead Slot", PowerTier: "D", Reason: "dead slot"},
		},
	}
	report := &FreyaReport{
		RemovalCount:      3, // gap
		NonLandTutorCount: 1, // gap (Combo bracket 4 wants 5)
		Profiles: []CardProfile{
			{Name: "Tranquil Cove", TypeLine: "Land", IsLand: true},
		},
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{RoleBoardWipe: 0},
		},
	}
	got := BuildSuggestedChanges(dp, report)

	// Verify adds sorted desc by priority
	for i := 1; i < len(got.Adds); i++ {
		if got.Adds[i-1].Priority < got.Adds[i].Priority {
			t.Errorf("adds not sorted desc by priority at i=%d: %d < %d",
				i, got.Adds[i-1].Priority, got.Adds[i].Priority)
		}
	}
	for i := 1; i < len(got.Cuts); i++ {
		if got.Cuts[i-1].Priority < got.Cuts[i].Priority {
			t.Errorf("cuts not sorted desc by priority at i=%d", i)
		}
	}
	for i := 1; i < len(got.Swaps); i++ {
		if got.Swaps[i-1].Priority < got.Swaps[i].Priority {
			t.Errorf("swaps not sorted desc by priority at i=%d", i)
		}
	}

	// TotalChanges should equal sum
	if got.TotalChanges != len(got.Adds)+len(got.Cuts)+len(got.Swaps) {
		t.Errorf("TotalChanges mismatch: %d vs %d+%d+%d",
			got.TotalChanges, len(got.Adds), len(got.Cuts), len(got.Swaps))
	}

	// Summary should mention counts and highest priority
	if !strings.Contains(got.Summary, "suggested") {
		t.Errorf("Summary missing 'suggested': %q", got.Summary)
	}

	// Highest priority across all 3 slices should be the D-grade
	// mana-base swap (priority 9).
	if len(got.Swaps) > 0 && got.Swaps[0].Priority != 9 {
		t.Errorf("D-grade swap priority: want 9, got %d", got.Swaps[0].Priority)
	}
}

func addCardNames(adds []SuggestedAdd) []string {
	out := make([]string, len(adds))
	for i, a := range adds {
		out[i] = a.CardName
	}
	return out
}

func cutNames(cuts []SuggestedCut) []string {
	out := make([]string, len(cuts))
	for i, c := range cuts {
		out[i] = c.CardName
	}
	return out
}
