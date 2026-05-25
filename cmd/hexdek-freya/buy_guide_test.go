package main

import (
	"strings"
	"testing"
)

// staticPrices returns a PriceLookup that resolves a fixed map.
// Cards absent from the map return ok=false so tests can exercise
// the unpriceable-card path.
func staticPrices(prices map[string]float64) PriceLookup {
	return func(name string) (float64, bool) {
		v, ok := prices[name]
		return v, ok
	}
}

// buyTestProfile builds a minimal DeckProfile + FreyaReport with the
// signals BuildBuyGuide reads. Defaults to a 4-color B4 deck that's
// thin on every density signal (so every gap category fires unless
// the caller overrides).
type buyTestSpec struct {
	colors        []string
	bracket       int
	primary       string
	rampCount     int
	drawCount     int
	tutorCount    int
	removalCount  int
	counterspells int
	boardWipes    int
	manaGrade     string
	owned         []string // cards already in the deck
	cuttable      []string // cards available as suggested cuts (in priority order)
}

func makeBuyTest(spec buyTestSpec) (*DeckProfile, *FreyaReport) {
	dp := &DeckProfile{
		ColorIdentity:    spec.colors,
		Bracket:          spec.bracket,
		PrimaryArchetype: spec.primary,
		RampCount:        spec.rampCount,
		DrawCount:        spec.drawCount,
		ManaBaseGrade:    spec.manaGrade,
	}
	for _, c := range spec.cuttable {
		dp.CuttableCards = append(dp.CuttableCards, CardQuality{Name: c, Tier: "cuttable"})
	}
	report := &FreyaReport{
		NonLandTutorCount: spec.tutorCount,
		RemovalCount:      spec.removalCount,
		Roles: &RoleAnalysis{
			RoleCounts: map[RoleTag]int{
				RoleCounterspell: spec.counterspells,
				RoleBoardWipe:    spec.boardWipes,
			},
		},
	}
	for _, name := range spec.owned {
		report.Profiles = append(report.Profiles, CardProfile{Name: name, IsLand: false})
	}
	return dp, report
}

// hasAdd returns the candidate matching the given add name, or nil.
func hasAdd(picks []BuyCandidate, name string) *BuyCandidate {
	for i := range picks {
		if picks[i].AddCard == name {
			return &picks[i]
		}
	}
	return nil
}

// TestBuildBuyGuide_HeadlineBudgetKnapsack pins the headline use case:
// a deck thin on ramp + draw + interaction gets a budget-fitting buy
// list ranked by impact-per-dollar. Cheap-high-impact picks (Sol Ring
// at $2 / 95 impact) MUST land before expensive-equivalent-impact
// picks (Mana Crypt at $200 / 95 impact).
func TestBuildBuyGuide_HeadlineBudgetKnapsack(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U", "B", "G"}, bracket: 4,
		primary: "Combo", rampCount: 4, drawCount: 4,
		tutorCount: 1, removalCount: 3, counterspells: 1,
		boardWipes: 0,
		cuttable: []string{"Bad Card 1", "Bad Card 2", "Bad Card 3", "Bad Card 4", "Bad Card 5", "Bad Card 6"},
	})
	prices := staticPrices(map[string]float64{
		"Sol Ring":             2.0,
		"Arcane Signet":        1.50,
		"Mana Crypt":           200.0,
		"Rhystic Study":        45.0,
		"Mystic Remora":        12.0,
		"Swords to Plowshares": 2.0,
		"Counterspell":         3.0,
		"Toxic Deluge":         18.0,
		"Demonic Tutor":        35.0,
		"Mystical Tutor":       12.0,
		// Cuttable card prices (recouped value).
		"Bad Card 1": 0.50, "Bad Card 2": 1.00, "Bad Card 3": 0.25,
		"Bad Card 4": 0.10, "Bad Card 5": 5.00, "Bad Card 6": 0.50,
	})
	guide := BuildBuyGuide(dp, rep, 50.0, prices)

	if len(guide.Suggested) == 0 {
		t.Fatal("expected at least one suggested buy, got none")
	}
	// Sol Ring + Swords to Plowshares + Arcane Signet must land —
	// they're cheap and high-impact, dominating the per-dollar rank.
	for _, must := range []string{"Sol Ring", "Swords to Plowshares", "Arcane Signet"} {
		if hasAdd(guide.Suggested, must) == nil {
			t.Errorf("expected %q in budget-fitting picks; got %v", must, addNames(guide.Suggested))
		}
	}
	// Mana Crypt (95 impact, $200) is much worse per-dollar than Sol
	// Ring (95 impact, $2). It must NOT land within a $50 budget.
	if hasAdd(guide.Suggested, "Mana Crypt") != nil {
		t.Errorf("Mana Crypt @ $200 should not fit in $50 budget; got %v", addNames(guide.Suggested))
	}
	// SpentTotal cannot exceed budget.
	if guide.SpentTotal > guide.Budget {
		t.Errorf("SpentTotal $%.2f exceeds budget $%.2f", guide.SpentTotal, guide.Budget)
	}
	// First N picks (where N ≤ len(cuttables)) get SuggestedCut
	// assignments from the cuttable pool. Picks beyond the pool
	// stay cutless (the user has to manually decide what to cut).
	cuttableCount := len(dp.CuttableCards)
	for i, c := range guide.Suggested {
		if i < cuttableCount && c.SuggestedCut == "" {
			t.Errorf("buy %q (rank %d ≤ cuttable count %d) should have a SuggestedCut",
				c.AddCard, i, cuttableCount)
		}
	}
}

// TestBuildBuyGuide_ColorIdentityFilter pins that candidates outside
// the deck's color identity are excluded. A mono-red deck can't run
// Counterspell / Swords to Plowshares / Demonic Tutor.
func TestBuildBuyGuide_ColorIdentityFilter(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"R"}, bracket: 3,
		primary: "Aggro / Go Wide",
		rampCount: 4, drawCount: 4,
		tutorCount: 0, removalCount: 3, counterspells: 0,
		cuttable: []string{"Bad Card 1", "Bad Card 2", "Bad Card 3"},
	})
	prices := staticPrices(map[string]float64{
		"Sol Ring": 2.0, "Arcane Signet": 1.50,
		"Counterspell": 3.0, "Rhystic Study": 45.0,
		"Swords to Plowshares": 2.0, "Demonic Tutor": 35.0,
		"Wheel of Fortune": 60.0,
		"Bad Card 1":       0.50, "Bad Card 2": 1.00, "Bad Card 3": 0.25,
	})
	guide := BuildBuyGuide(dp, rep, 100.0, prices)
	for _, c := range guide.AllCandidates {
		// No candidate should require U/W/B/G in a mono-red deck.
		for _, col := range c.Colors {
			if col != "R" && col != "" {
				if !colorIdentityAllows(dp.ColorIdentity, c.Colors) {
					t.Errorf("candidate %q (colors %v) shouldn't pass mono-red identity gate",
						c.AddCard, c.Colors)
				}
			}
		}
	}
	// Sol Ring (colorless) and Wheel of Fortune (red) should still
	// be eligible.
	if !contains(guideAdds(guide), "Sol Ring") {
		t.Errorf("Sol Ring (colorless) should be a candidate in any color identity")
	}
	if !contains(guideAdds(guide), "Wheel of Fortune") {
		t.Errorf("Wheel of Fortune (red) should be a candidate in mono-red")
	}
	// Pure-blue card should NOT appear.
	if contains(guideAdds(guide), "Counterspell") {
		t.Errorf("Counterspell (blue) shouldn't be a candidate in mono-red, got %v",
			guideAdds(guide))
	}
}

// TestBuildBuyGuide_OwnedCardsSkipped pins that cards already in the
// deck are excluded — don't suggest buying Sol Ring if it's already
// in the list.
func TestBuildBuyGuide_OwnedCardsSkipped(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U"}, bracket: 3,
		rampCount: 4, drawCount: 4, removalCount: 3,
		owned:    []string{"Sol Ring", "Swords to Plowshares"},
		cuttable: []string{"Cut1", "Cut2", "Cut3"},
	})
	prices := staticPrices(map[string]float64{
		"Sol Ring": 2.0, "Arcane Signet": 1.50,
		"Swords to Plowshares": 2.0, "Path to Exile": 8.0,
		"Counterspell": 3.0,
		"Cut1":         0.50, "Cut2": 1.0, "Cut3": 0.25,
	})
	guide := BuildBuyGuide(dp, rep, 50.0, prices)
	for _, c := range guide.AllCandidates {
		if c.AddCard == "Sol Ring" || c.AddCard == "Swords to Plowshares" {
			t.Errorf("owned card %q should be skipped, got it in candidates", c.AddCard)
		}
	}
	// Path to Exile (W) isn't owned and the deck plays W — should appear.
	if !contains(guideAdds(guide), "Path to Exile") {
		t.Errorf("expected Path to Exile (unowned W card) in candidates, got %v",
			guideAdds(guide))
	}
}

// TestBuildBuyGuide_WellTunedDeckReturnsEmpty pins the no-noise
// guarantee: a deck that clears every threshold has zero gaps, so
// the buy guide has nothing to suggest. The Decks screen renders
// "no upgrades needed" rather than fabricated picks.
func TestBuildBuyGuide_WellTunedDeckReturnsEmpty(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U", "B", "G"}, bracket: 4,
		primary: "Midrange",
		rampCount: 12, drawCount: 12,
		tutorCount: 6, removalCount: 8, counterspells: 4,
		boardWipes: 2, manaGrade: "A",
		cuttable: []string{"X1", "X2"},
	})
	prices := staticPrices(map[string]float64{})
	guide := BuildBuyGuide(dp, rep, 100.0, prices)
	if len(guide.Suggested) > 0 {
		t.Errorf("well-tuned deck should have empty Suggested, got %v", addNames(guide.Suggested))
	}
	if len(guide.AllCandidates) > 0 {
		t.Errorf("well-tuned deck should have empty AllCandidates, got %v", addNames(guide.AllCandidates))
	}
}

// TestBuildBuyGuide_OverBudgetGoesToWishlist pins the SkippedOverBudget
// list: candidates that would have ranked into Suggested but didn't
// fit at allocation time get moved to the wishlist so the UI can show
// "if you had $X more you'd want Y."
func TestBuildBuyGuide_OverBudgetGoesToWishlist(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"U"}, bracket: 5,
		primary: "Combo",
		rampCount: 4, drawCount: 4,
		tutorCount: 0, removalCount: 0, counterspells: 0,
		boardWipes: 0,
		cuttable:   []string{"Cut1", "Cut2"},
	})
	// Tiny budget — only the cheapest pick fits.
	prices := staticPrices(map[string]float64{
		"Sol Ring":     2.0,
		"Mana Crypt":   200.0,
		"Rhystic Study": 45.0,
		"Cut1":         0,
		"Cut2":         0,
	})
	guide := BuildBuyGuide(dp, rep, 5.0, prices)
	if len(guide.Suggested) > 1 {
		// Only Sol Ring at $2 fits in $5.
		t.Errorf("only Sol Ring should fit in $5 budget, got %v", addNames(guide.Suggested))
	}
	// Mana Crypt / Rhystic Study should be in SkippedOverBudget.
	skippedNames := addNames(guide.SkippedOverBudget)
	if !contains(skippedNames, "Mana Crypt") || !contains(skippedNames, "Rhystic Study") {
		t.Errorf("expected over-budget candidates in wishlist, got %v", skippedNames)
	}
}

// TestBuildBuyGuide_NetCostAccountsForCutValue pins that suggested
// cuts subtract their resale value from the candidate's NetCost.
// A $50 add paired with a $10 cut nets to $40, not $50.
func TestBuildBuyGuide_NetCostAccountsForCutValue(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"U"}, bracket: 4,
		rampCount: 4, drawCount: 4, removalCount: 3,
		cuttable: []string{"Expensive Cut"},
	})
	prices := staticPrices(map[string]float64{
		"Rhystic Study": 45.0,
		"Expensive Cut": 15.0,
	})
	guide := BuildBuyGuide(dp, rep, 100.0, prices)
	rhystic := hasAdd(guide.AllCandidates, "Rhystic Study")
	if rhystic == nil {
		t.Fatalf("expected Rhystic Study in candidates, got %v", addNames(guide.AllCandidates))
	}
	if rhystic.SuggestedCut != "Expensive Cut" {
		t.Errorf("expected suggested cut 'Expensive Cut', got %q", rhystic.SuggestedCut)
	}
	wantNet := 45.0 - 15.0
	if rhystic.NetCost != wantNet {
		t.Errorf("NetCost = %.2f, want %.2f (add $45 − cut $15)", rhystic.NetCost, wantNet)
	}
}

// TestBuildBuyGuide_UnpriceableCardNotedNotDropped pins that cards
// the lookup couldn't price still appear in the guide (with
// AddPriceKnown=false) so the user sees them; they just default to
// $0 and surface in UnpriceableNotes. Without this, missing price
// data would silently hide candidates.
func TestBuildBuyGuide_UnpriceableCardNotedNotDropped(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W"}, bracket: 3,
		rampCount: 4, drawCount: 4, removalCount: 3,
		cuttable: []string{"Cut1"},
	})
	// Only price one card; the rest return ok=false.
	prices := staticPrices(map[string]float64{
		"Sol Ring": 2.0,
		"Cut1":     0,
	})
	guide := BuildBuyGuide(dp, rep, 100.0, prices)

	// Swords to Plowshares should still be a candidate (it's a W
	// interaction card and addresses the interaction gap) even
	// without a price.
	stp := hasAdd(guide.AllCandidates, "Swords to Plowshares")
	if stp == nil {
		t.Fatalf("Swords to Plowshares should appear in candidates even without price data")
	}
	if stp.AddPriceKnown {
		t.Errorf("expected AddPriceKnown=false for unpriceable card, got true")
	}
	if len(guide.UnpriceableNotes) == 0 {
		t.Errorf("expected UnpriceableNotes to list cards with missing prices")
	}
}

// TestBuildBuyGuide_NilPriceLookupStillWorks pins that callers in
// static-mode (no DB / no API) can pass nil and get back a candidate
// list ranked by impact alone (NetCost=0, ImpactPerDollar = Impact).
func TestBuildBuyGuide_NilPriceLookupStillWorks(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U"}, bracket: 4,
		rampCount: 4, drawCount: 4, removalCount: 3,
		cuttable: []string{"Cut1"},
	})
	guide := BuildBuyGuide(dp, rep, 100.0, nil)
	if len(guide.AllCandidates) == 0 {
		t.Fatalf("nil prices should still produce candidates ranked by impact")
	}
	for _, c := range guide.AllCandidates {
		if c.AddPriceKnown {
			t.Errorf("nil prices should leave AddPriceKnown=false, got true on %s", c.AddCard)
		}
	}
}

// TestBuildBuyGuide_NilInputsSafe pins safe-default contracts.
func TestBuildBuyGuide_NilInputsSafe(t *testing.T) {
	g := BuildBuyGuide(nil, &FreyaReport{}, 100, nil)
	if g == nil || len(g.Suggested) > 0 {
		t.Errorf("nil deck profile should return empty guide, got %+v", g)
	}
	g = BuildBuyGuide(&DeckProfile{}, nil, 100, nil)
	if g == nil || len(g.Suggested) > 0 {
		t.Errorf("nil report should return empty guide, got %+v", g)
	}
}

// TestBuildBuyGuide_TutorGateRespectsArchetype pins the same archetype
// gating coaching.go uses: only Combo / Storm decks at bracket 4+ get
// tutor candidates. A Voltron deck should not be told to buy Demonic
// Tutor even at B5.
func TestBuildBuyGuide_TutorGateRespectsArchetype(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "B"}, bracket: 5,
		primary: "Voltron",
		rampCount: 12, drawCount: 11, removalCount: 6, counterspells: 4,
		boardWipes: 2, tutorCount: 1,
		cuttable: []string{"Cut1"},
	})
	prices := staticPrices(map[string]float64{
		"Demonic Tutor": 35.0, "Vampiric Tutor": 30.0,
		"Cut1":          0,
	})
	guide := BuildBuyGuide(dp, rep, 100.0, prices)
	for _, c := range guide.AllCandidates {
		if c.Category == "tutors" {
			t.Errorf("Voltron deck shouldn't get tutor candidates, got %q", c.AddCard)
		}
	}
}

// TestBuildBuyGuide_AlphabeticalTiebreakStable pins that candidates
// with identical ImpactPerDollar sort alphabetically by AddCard for
// deterministic ordering across runs (guards against map-iter noise
// in the staples catalog).
func TestBuildBuyGuide_AlphabeticalTiebreakStable(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U"}, bracket: 3,
		rampCount: 4, drawCount: 4, removalCount: 3,
		cuttable: []string{"Cut1"},
	})
	// Identical $5 price for everything → impact alone drives rank,
	// alphabetical tiebreaks within each impact tier.
	prices := staticPrices(map[string]float64{
		"Sol Ring": 5.0, "Arcane Signet": 5.0, "Mana Crypt": 5.0,
		"Mox Diamond": 5.0, "Chrome Mox": 5.0,
		"Cut1":        0,
	})
	guide1 := BuildBuyGuide(dp, rep, 100.0, prices)
	guide2 := BuildBuyGuide(dp, rep, 100.0, prices)
	if len(guide1.AllCandidates) != len(guide2.AllCandidates) {
		t.Fatalf("non-deterministic length: %d vs %d",
			len(guide1.AllCandidates), len(guide2.AllCandidates))
	}
	for i := range guide1.AllCandidates {
		if guide1.AllCandidates[i].AddCard != guide2.AllCandidates[i].AddCard {
			t.Errorf("non-deterministic order at [%d]: %q vs %q",
				i, guide1.AllCandidates[i].AddCard, guide2.AllCandidates[i].AddCard)
		}
	}
}

// TestFormatBuyGuide_RendersSections pins the text output contains
// every Decks-screen panel section: header with budget, picked list
// with prices + impact + cut, wishlist with over-budget items.
func TestFormatBuyGuide_RendersSections(t *testing.T) {
	dp, rep := makeBuyTest(buyTestSpec{
		colors: []string{"W", "U"}, bracket: 4,
		rampCount: 4, drawCount: 4, removalCount: 3,
		cuttable: []string{"Cut1", "Cut2", "Cut3"},
	})
	prices := staticPrices(map[string]float64{
		"Sol Ring":      2.0,
		"Mana Crypt":    200.0,
		"Rhystic Study": 45.0,
		"Cut1":          0.50, "Cut2": 1.0, "Cut3": 0.25,
	})
	guide := BuildBuyGuide(dp, rep, 10.0, prices)
	out := FormatBuyGuide(guide)
	for _, want := range []string{
		"Buy Guide", "budget: $10.00",
		"Sol Ring",
		"impact",
		"cut Cut1",
		"Wishlist",
		"Mana Crypt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatBuyGuide output missing %q. Got:\n%s", want, out)
		}
	}
}

func addNames(picks []BuyCandidate) []string {
	out := make([]string, 0, len(picks))
	for _, p := range picks {
		out = append(out, p.AddCard)
	}
	return out
}

func guideAdds(g *BuyGuide) []string {
	if g == nil {
		return nil
	}
	return addNames(g.AllCandidates)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
