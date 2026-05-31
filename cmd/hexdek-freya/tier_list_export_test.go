package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// TierListExport tests. Each fixture hand-constructs a synthetic
// corpus of *FreyaReport with explicit Profiles + Archetype.Primary
// + Bracket so the ranking math is isolated from the deck-analysis
// pipeline.
// ---------------------------------------------------------------------------

func deckReport(archetype string, bracket int, cards ...string) *FreyaReport {
	profiles := make([]CardProfile, len(cards))
	for i, name := range cards {
		profiles[i] = CardProfile{Name: name}
	}
	r := &FreyaReport{
		Profiles:  profiles,
		Archetype: &ArchetypeClassification{Primary: archetype, Bracket: bracket},
	}
	return r
}

func findArchetype(t *testing.T, tl *TierListExport, arch string) *ArchetypeTierList {
	t.Helper()
	for i := range tl.Archetypes {
		if tl.Archetypes[i].Archetype == arch {
			return &tl.Archetypes[i]
		}
	}
	t.Fatalf("archetype %q not in tier list (got %+v)", arch, archetypeNamesOf(tl))
	return nil
}

func archetypeNamesOf(tl *TierListExport) []string {
	out := make([]string, len(tl.Archetypes))
	for i, a := range tl.Archetypes {
		out[i] = a.Archetype
	}
	return out
}

func findCard(t *testing.T, list *ArchetypeTierList, name string) *TierCard {
	t.Helper()
	for i := range list.TopCards {
		if list.TopCards[i].Name == name {
			return &list.TopCards[i]
		}
	}
	t.Fatalf("card %q not in %s tier list (got %d cards)", name, list.Archetype, len(list.TopCards))
	return nil
}

// TestTierList_VoltronAutoInclude: 5-deck Voltron cohort with a
// shared "Sword of Feast and Famine" auto-include card in every deck
// should put that card at the top (InclusionRate=1.0).
func TestTierList_VoltronAutoInclude(t *testing.T) {
	reports := []*FreyaReport{
		deckReport("Voltron", 3, "Sword of Feast and Famine", "Card A"),
		deckReport("Voltron", 4, "Sword of Feast and Famine", "Card B"),
		deckReport("Voltron", 4, "Sword of Feast and Famine", "Card C"),
		deckReport("Voltron", 3, "Sword of Feast and Famine", "Card D"),
		deckReport("Voltron", 4, "Sword of Feast and Famine", "Card E"),
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Voltron")
	if v.DeckCount != 5 {
		t.Errorf("DeckCount: got %d, want 5", v.DeckCount)
	}
	if len(v.TopCards) == 0 {
		t.Fatal("expected at least 1 top card")
	}
	if v.TopCards[0].Name != "Sword of Feast and Famine" {
		t.Errorf("top card: got %q, want \"Sword of Feast and Famine\"", v.TopCards[0].Name)
	}
	if v.TopCards[0].InclusionRate != 1.0 {
		t.Errorf("InclusionRate: got %.2f, want 1.00", v.TopCards[0].InclusionRate)
	}
	if v.TopCards[0].InclusionCount != 5 {
		t.Errorf("InclusionCount: got %d, want 5", v.TopCards[0].InclusionCount)
	}
}

// TestTierList_SkipSmallCohorts: archetypes with < tierListMinArchetypeDecks
// decks are dropped entirely.
func TestTierList_SkipSmallCohorts(t *testing.T) {
	reports := []*FreyaReport{
		deckReport("Voltron", 3, "A", "B"),
		deckReport("Voltron", 3, "A", "B"),
		// Stax has only 2 decks → below cohort floor (5).
		deckReport("Stax", 4, "Cursed Totem"),
		deckReport("Stax", 4, "Cursed Totem"),
	}
	tl := ComputeTierListExport(reports)
	for _, a := range tl.Archetypes {
		if a.Archetype == "Stax" {
			t.Errorf("Stax (2-deck cohort) should be filtered out, got %+v", a)
		}
		if a.Archetype == "Voltron" {
			t.Errorf("Voltron (2-deck cohort) should be filtered out, got %+v", a)
		}
	}
}

// TestTierList_SkipLowInclusion: cards appearing in < tierListMinDeckCount
// decks within an archetype are dropped.
func TestTierList_SkipLowInclusion(t *testing.T) {
	reports := []*FreyaReport{
		deckReport("Combo", 4, "Shared", "Lone1"),
		deckReport("Combo", 4, "Shared", "Lone2"),
		deckReport("Combo", 4, "Shared", "Lone3"),
		deckReport("Combo", 4, "Shared", "Lone4"),
		deckReport("Combo", 4, "Shared", "Lone5"),
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Combo")
	// "Shared" (5 decks) should be ranked. "Lone1".."Lone5" each appear
	// in 1 deck — below the gate.
	for _, c := range v.TopCards {
		if c.Name == "Lone1" || c.Name == "Lone5" {
			t.Errorf("single-deck card %q should be filtered out, got %+v", c.Name, c)
		}
	}
	if len(v.TopCards) != 1 {
		t.Errorf("expected exactly 1 ranked card (Shared), got %d", len(v.TopCards))
	}
}

// TestTierList_WinImpactCorrelation: a card that appears in
// higher-bracket decks gets a positive WinImpact and ranks higher than
// an equally-included card that appears in lower-bracket decks.
func TestTierList_WinImpactCorrelation(t *testing.T) {
	// "HighBracket" appears in 3 B4 decks. "LowBracket" appears in 3 B2
	// decks. Both have InclusionRate = 0.5 (3 of 6 decks). HighBracket
	// should rank above LowBracket.
	reports := []*FreyaReport{
		deckReport("Midrange", 4, "HighBracket", "Filler1"),
		deckReport("Midrange", 4, "HighBracket", "Filler2"),
		deckReport("Midrange", 4, "HighBracket", "Filler3"),
		deckReport("Midrange", 2, "LowBracket", "Filler4"),
		deckReport("Midrange", 2, "LowBracket", "Filler5"),
		deckReport("Midrange", 2, "LowBracket", "Filler6"),
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Midrange")
	high := findCard(t, v, "HighBracket")
	low := findCard(t, v, "LowBracket")
	if high.WinImpact <= 0 {
		t.Errorf("HighBracket WinImpact should be positive (appears in B4 vs B2 cohort), got %.2f",
			high.WinImpact)
	}
	if low.WinImpact >= 0 {
		t.Errorf("LowBracket WinImpact should be negative, got %.2f", low.WinImpact)
	}
	if high.TierScore <= low.TierScore {
		t.Errorf("HighBracket TierScore (%.2f) should exceed LowBracket (%.2f)",
			high.TierScore, low.TierScore)
	}
}

// TestTierList_LandsExcluded: lands are skipped from per-archetype
// ranking (too generic).
func TestTierList_LandsExcluded(t *testing.T) {
	reports := []*FreyaReport{}
	for i := 0; i < 5; i++ {
		r := &FreyaReport{
			Profiles: []CardProfile{
				{Name: "Plains", IsLand: true},
				{Name: "Sol Ring"},
			},
			Archetype: &ArchetypeClassification{Primary: "Voltron", Bracket: 3},
		}
		reports = append(reports, r)
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Voltron")
	for _, c := range v.TopCards {
		if c.Name == "Plains" {
			t.Errorf("Plains (land) should be excluded from tier list, got %+v", c)
		}
	}
}

// TestTierList_NilDefensive: nil / empty inputs return zero-valued
// export without panicking.
func TestTierList_NilDefensive(t *testing.T) {
	if tl := ComputeTierListExport(nil); tl == nil {
		t.Error("expected non-nil zero-value, got nil")
	} else {
		if tl.CorpusSize != 0 || tl.ArchetypeCount != 0 {
			t.Errorf("expected zero-value, got CorpusSize=%d ArchCount=%d",
				tl.CorpusSize, tl.ArchetypeCount)
		}
	}
	if tl := ComputeTierListExport([]*FreyaReport{nil, nil}); tl.CorpusSize != 2 {
		t.Errorf("expected CorpusSize=2 even with nil reports, got %d", tl.CorpusSize)
	}
}

// TestTierList_DeterministicOrdering: archetypes alphabetical;
// within an archetype, equal-score cards break ties by InclusionCount
// then name.
func TestTierList_DeterministicOrdering(t *testing.T) {
	// "Zebra" and "Alpha" both included in all 5 decks at the same
	// bracket → equal TierScore, equal InclusionCount → tie-break by
	// name ascending = "Alpha" first.
	reports := []*FreyaReport{}
	for i := 0; i < 5; i++ {
		reports = append(reports,
			deckReport("Combo", 3, "Zebra", "Alpha"))
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Combo")
	if len(v.TopCards) < 2 {
		t.Fatalf("expected 2 cards, got %d", len(v.TopCards))
	}
	if v.TopCards[0].Name != "Alpha" {
		t.Errorf("first card on tie: got %q, want \"Alpha\" (name asc tiebreak)",
			v.TopCards[0].Name)
	}
}

// TestTierList_TopN: only top N cards per archetype emitted.
func TestTierList_TopN(t *testing.T) {
	// Construct an archetype with 60 ranked cards; tier list should
	// cap at tierListTopN (50).
	reports := []*FreyaReport{}
	cardNames := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		cardNames = append(cardNames, "Card_"+string(rune('A'+i%26))+string(rune('a'+i/26)))
	}
	// 5 decks each with all 60 cards.
	for i := 0; i < 5; i++ {
		reports = append(reports, deckReport("Midrange", 3, cardNames...))
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Midrange")
	if len(v.TopCards) != tierListTopN {
		t.Errorf("len(TopCards): got %d, want %d", len(v.TopCards), tierListTopN)
	}
}

// TestTierList_AvgBracketComputed: cohort avg bracket is the mean
// across decks in the archetype.
func TestTierList_AvgBracketComputed(t *testing.T) {
	reports := []*FreyaReport{
		deckReport("Voltron", 2, "A"),
		deckReport("Voltron", 3, "A"),
		deckReport("Voltron", 4, "A"),
		deckReport("Voltron", 3, "A"),
		deckReport("Voltron", 3, "A"),
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Voltron")
	if v.AvgBracket < 2.99 || v.AvgBracket > 3.01 {
		t.Errorf("AvgBracket: got %.2f, want ≈3.00", v.AvgBracket)
	}
}

// TestTierList_SkipNoArchetype: reports without an archetype primary
// are skipped (can't bucket).
func TestTierList_SkipNoArchetype(t *testing.T) {
	reports := []*FreyaReport{
		{Profiles: []CardProfile{{Name: "Card"}}, Archetype: nil},
		{Profiles: []CardProfile{{Name: "Card"}}, Archetype: &ArchetypeClassification{Primary: ""}},
	}
	// Plus 5 well-formed ones to satisfy cohort floor for "Voltron".
	for i := 0; i < 5; i++ {
		reports = append(reports, deckReport("Voltron", 3, "Card"))
	}
	tl := ComputeTierListExport(reports)
	v := findArchetype(t, tl, "Voltron")
	if v.DeckCount != 5 {
		t.Errorf("DeckCount: got %d, want 5 (nil/empty archetype reports skipped)",
			v.DeckCount)
	}
}
