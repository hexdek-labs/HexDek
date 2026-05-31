package huginn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Recommender tests — Partners + Extend queries
//
// Fixtures use a Niv-Mizzet-anchored corpus: the canonical "spell cast →
// draw card" pattern with the three real-world partners (Curiosity,
// Tandem Lookout, Ophidian Eye) pinned at Tier 3, plus a couple of
// distractor interactions at lower tiers to verify the minTier filter.
// ---------------------------------------------------------------------------

func nivMizzetFixture() []LearnedInteraction {
	return []LearnedInteraction{
		// Headline Niv-Mizzet draw triggers — all three canonical partners.
		{
			Pattern:          "creature deals damage → draw card",
			ExampleCards:     []string{"Curiosity + Niv-Mizzet, Parun", "Niv-Mizzet, Parun + Tandem Lookout", "Niv-Mizzet, Parun + Ophidian Eye"},
			ObservationCount: 24,
			UniqueDeckCount:  8,
			AvgImpactScore:   12.4,
			Tier:             TierConfirmed,
		},
		// Lower-tier distractor: Niv pairing surfaces in casual decks, not
		// in the headline pattern. minTier=TierRecurring should still
		// surface it; minTier=TierConfirmed should exclude.
		{
			Pattern:          "instant cast → opponent loses life",
			ExampleCards:     []string{"Niv-Mizzet, Parun + Manabarbs"},
			ObservationCount: 4,
			UniqueDeckCount:  2,
			AvgImpactScore:   3.1,
			Tier:             TierRecurring,
		},
		// Tier 1 noise — should NEVER appear in default queries.
		{
			Pattern:          "creature ETB → flavor text triggered",
			ExampleCards:     []string{"Niv-Mizzet, Parun + Llanowar Elves"},
			ObservationCount: 1,
			UniqueDeckCount:  1,
			AvgImpactScore:   0.5,
			Tier:             TierObserved,
		},
		// Unrelated pattern — should NOT appear in Niv queries.
		{
			Pattern:          "produces mana → consumes mana",
			ExampleCards:     []string{"Sol Ring + Selvala, Heart of the Wilds"},
			ObservationCount: 12,
			UniqueDeckCount:  6,
			AvgImpactScore:   6.5,
			Tier:             TierConfirmed,
		},
	}
}

func TestPartners_NivMizzetSurfacesAllThreeCanonicalPartners(t *testing.T) {
	hits := partnersFromInteractions(nivMizzetFixture(), "Niv-Mizzet, Parun", TierConfirmed)
	wantPartners := map[string]bool{
		"Curiosity":      false,
		"Tandem Lookout": false,
		"Ophidian Eye":   false,
	}
	for _, h := range hits {
		if _, ok := wantPartners[h.Partner]; ok {
			wantPartners[h.Partner] = true
		}
	}
	for p, found := range wantPartners {
		if !found {
			t.Errorf("expected Niv-Mizzet partner %q in Tier-3 results; got %d hits: %+v", p, len(hits), hits)
		}
	}
	// Tier 3 only — Manabarbs is Tier 2 and should be filtered out.
	for _, h := range hits {
		if h.Partner == "Manabarbs" {
			t.Errorf("Manabarbs is Tier 2; should be filtered at TierConfirmed minTier")
		}
	}
}

func TestPartners_MinTierRecurringIncludesTier2(t *testing.T) {
	hits := partnersFromInteractions(nivMizzetFixture(), "Niv-Mizzet, Parun", TierRecurring)
	foundManabarbs := false
	for _, h := range hits {
		if h.Partner == "Manabarbs" {
			foundManabarbs = true
		}
	}
	if !foundManabarbs {
		t.Errorf("Manabarbs (Tier 2) should appear when minTier=Recurring")
	}
	// Tier 1 noise still filtered out.
	for _, h := range hits {
		if h.Partner == "Llanowar Elves" {
			t.Errorf("Llanowar Elves (Tier 1) should NOT appear at minTier=Recurring")
		}
	}
}

func TestPartners_RankedByTierThenScore(t *testing.T) {
	hits := partnersFromInteractions(nivMizzetFixture(), "Niv-Mizzet, Parun", TierRecurring)
	if len(hits) < 2 {
		t.Fatalf("expected ≥ 2 hits for ranked sort check, got %d", len(hits))
	}
	// First hit should be Tier 3 (canonical partners), not Tier 2 Manabarbs.
	if hits[0].Tier != TierConfirmed {
		t.Errorf("expected top hit at Tier 3, got Tier %d (%s)", hits[0].Tier, hits[0].Partner)
	}
	// All Tier 3 hits should sort before any Tier 2.
	sawTier2 := false
	for _, h := range hits {
		if h.Tier == TierRecurring {
			sawTier2 = true
		} else if h.Tier == TierConfirmed && sawTier2 {
			t.Errorf("Tier 3 hit %q appeared after a Tier 2 — tier sort broken", h.Partner)
		}
	}
}

func TestPartners_QueryIsCaseInsensitive(t *testing.T) {
	hits := partnersFromInteractions(nivMizzetFixture(), "niv-mizzet, parun", TierConfirmed)
	if len(hits) < 3 {
		t.Errorf("case-insensitive query should match canonical 3-partner Niv set, got %d hits", len(hits))
	}
}

func TestPartners_UnknownCardReturnsEmpty(t *testing.T) {
	hits := partnersFromInteractions(nivMizzetFixture(), "Card That Doesn't Exist", TierConfirmed)
	if len(hits) != 0 {
		t.Errorf("unknown card should return 0 hits, got %d", len(hits))
	}
}

func TestPartners_PartnerExposesAllSharedPatterns(t *testing.T) {
	multi := []LearnedInteraction{
		{
			Pattern:          "creature deals damage → draw card",
			ExampleCards:     []string{"Curiosity + Niv-Mizzet, Parun"},
			ObservationCount: 24, UniqueDeckCount: 8, AvgImpactScore: 12.4, Tier: TierConfirmed,
		},
		{
			Pattern:          "spell cast → counter card",
			ExampleCards:     []string{"Curiosity + Niv-Mizzet, Parun"},
			ObservationCount: 8, UniqueDeckCount: 4, AvgImpactScore: 5.2, Tier: TierConfirmed,
		},
	}
	hits := partnersFromInteractions(multi, "Niv-Mizzet, Parun", TierConfirmed)
	if len(hits) != 1 {
		t.Fatalf("expected 1 partner (Curiosity), got %d", len(hits))
	}
	if hits[0].PairCount != 2 {
		t.Errorf("expected Curiosity to surface 2 distinct patterns, got %d", hits[0].PairCount)
	}
	if len(hits[0].Patterns) != 2 {
		t.Errorf("expected 2 patterns in result, got %d", len(hits[0].Patterns))
	}
	// Higher-impact pattern first.
	if hits[0].Patterns[0] != "creature deals damage → draw card" {
		t.Errorf("expected highest-impact pattern first, got %q", hits[0].Patterns[0])
	}
}

// ---------------------------------------------------------------------------
// Extend tests
// ---------------------------------------------------------------------------

func TestExtend_RecommendsCanonicalPartnersForNivDeck(t *testing.T) {
	// Deck has Niv-Mizzet but NONE of the canonical partners — Extend
	// should rank Curiosity / Tandem / Ophidian Eye at the top.
	deck := []string{
		"Niv-Mizzet, Parun",
		"Sol Ring",
		"Counterspell",
		"Mountain",
		"Island",
	}
	hits := extendFromInteractions(nivMizzetFixture(), deck, TierConfirmed)

	// All 3 Niv partners should appear; Sol Ring's pair partner Selvala
	// should also appear since Sol Ring is in deck and Selvala isn't.
	want := map[string]bool{
		"Curiosity":                  false,
		"Tandem Lookout":             false,
		"Ophidian Eye":               false,
		"Selvala, Heart of the Wilds": false,
	}
	for _, h := range hits {
		if _, ok := want[h.Card]; ok {
			want[h.Card] = true
		}
	}
	for c, found := range want {
		if !found {
			t.Errorf("Extend should recommend %q for Niv-Mizzet deck; got hits: %+v", c, hits)
		}
	}
	// None of the cards already in the deck should appear.
	for _, h := range hits {
		for _, d := range deck {
			if h.Card == d {
				t.Errorf("Extend returned in-deck card %q", h.Card)
			}
		}
	}
}

func TestExtend_RanksMultiPartnerCandidatesHigher(t *testing.T) {
	// Synthetic corpus where Card X pairs with TWO deck cards (high score)
	// and Card Y pairs with ONE deck card at the same per-pair score.
	corpus := []LearnedInteraction{
		{
			Pattern: "pattern X-A",
			ExampleCards: []string{"DeckCardA + CandidateX"},
			ObservationCount: 10, UniqueDeckCount: 4, AvgImpactScore: 6.0,
			Tier: TierConfirmed,
		},
		{
			Pattern: "pattern X-B",
			ExampleCards: []string{"DeckCardB + CandidateX"},
			ObservationCount: 10, UniqueDeckCount: 4, AvgImpactScore: 6.0,
			Tier: TierConfirmed,
		},
		{
			Pattern: "pattern Y-A",
			ExampleCards: []string{"DeckCardA + CandidateY"},
			ObservationCount: 10, UniqueDeckCount: 4, AvgImpactScore: 6.0,
			Tier: TierConfirmed,
		},
	}
	deck := []string{"DeckCardA", "DeckCardB"}
	hits := extendFromInteractions(corpus, deck, TierConfirmed)
	if len(hits) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(hits))
	}
	if hits[0].Card != "CandidateX" {
		t.Errorf("CandidateX (2 deck pairings) should rank first; got %q", hits[0].Card)
	}
	if hits[0].Pairs != 2 {
		t.Errorf("CandidateX Pairs should be 2, got %d", hits[0].Pairs)
	}
}

func TestExtend_EmptyDeckReturnsEmpty(t *testing.T) {
	if hits := extendFromInteractions(nivMizzetFixture(), nil, TierConfirmed); hits != nil {
		t.Errorf("nil deck should return nil hits, got %d", len(hits))
	}
}

func TestExtend_DoesNotRecommendCardsAlreadyInDeck(t *testing.T) {
	deck := []string{"Niv-Mizzet, Parun", "Curiosity", "Tandem Lookout", "Ophidian Eye"}
	hits := extendFromInteractions(nivMizzetFixture(), deck, TierConfirmed)
	for _, h := range hits {
		for _, d := range deck {
			if h.Card == d {
				t.Errorf("Extend returned %q which is already in deck", h.Card)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// File-backed integration
// ---------------------------------------------------------------------------

func TestPartners_ReadsLearnedJSONFromDisk(t *testing.T) {
	dir := t.TempDir()
	data, err := json.MarshalIndent(nivMizzetFixture(), "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, learnedFile), data, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	hits, err := Partners(dir, "Niv-Mizzet, Parun", TierConfirmed)
	if err != nil {
		t.Fatalf("Partners: %v", err)
	}
	if len(hits) < 3 {
		t.Errorf("disk-backed Partners should surface 3 Niv canonical partners, got %d", len(hits))
	}
}

func TestLoadDeckJSON_NormalizesCommanderAndLibrary(t *testing.T) {
	dir := t.TempDir()
	deck := DeckJSON{
		Commander: "Niv-Mizzet, Parun",
		Library:   []string{"Sol Ring", "Counterspell", "Counterspell"}, // dedup
	}
	data, _ := json.MarshalIndent(deck, "", "  ")
	path := filepath.Join(dir, "niv.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write deck: %v", err)
	}
	d, names, err := LoadDeckJSON(path)
	if err != nil {
		t.Fatalf("LoadDeckJSON: %v", err)
	}
	if d.Commander != "Niv-Mizzet, Parun" {
		t.Errorf("commander not preserved: %q", d.Commander)
	}
	// Commander + Sol Ring + Counterspell (deduped) = 3.
	if len(names) != 3 {
		t.Errorf("expected 3 deduped names (commander + 2 unique lib), got %d: %v", len(names), names)
	}
}
