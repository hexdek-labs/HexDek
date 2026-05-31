package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// renderHTMLTierListSection tests. Exercise the renderer directly with
// hand-constructed TierListExport + FreyaReport fixtures so behavior is
// isolated from --all-decks corpus generation.
// ---------------------------------------------------------------------------

func sampleTierList() *TierListExport {
	return &TierListExport{
		CorpusSize:     12,
		ArchetypeCount: 2,
		Archetypes: []ArchetypeTierList{
			{
				Archetype:  "Voltron",
				DeckCount:  7,
				AvgBracket: 3.4,
				TopCards: []TierCard{
					{
						Name: "Sword of Feast and Famine", InclusionCount: 6, InclusionRate: 6.0 / 7.0,
						AvgBracketWith: 3.6, AvgBracketWithout: 2.0, WinImpact: 1.6, TierScore: 2.23,
					},
					{
						Name: "Lightning Greaves", InclusionCount: 5, InclusionRate: 5.0 / 7.0,
						AvgBracketWith: 3.5, AvgBracketWithout: 2.8, WinImpact: 0.7, TierScore: 1.21,
					},
				},
			},
			{
				Archetype: "Combo",
				DeckCount: 5,
				TopCards: []TierCard{
					{Name: "Demonic Tutor", InclusionCount: 5, InclusionRate: 1.0, TierScore: 1.5},
				},
			},
		},
	}
}

func reportForArchetype(arch string) *FreyaReport {
	return &FreyaReport{
		DeckName: "Test Deck",
		Profile:  &DeckProfile{PrimaryArchetype: arch},
	}
}

// TestRenderHTMLTierList_MatchRenders: matching archetype renders the
// collapsible section with the expected meta line, table, and Scryfall
// card links.
func TestRenderHTMLTierList_MatchRenders(t *testing.T) {
	r := reportForArchetype("Voltron")
	tl := sampleTierList()
	var buf bytes.Buffer
	renderHTMLTierListSection(&buf, r, tl)
	out := buf.String()

	musts := []string{
		"<details>",
		"<summary><h2>Auto-includes for Voltron",
		"Ranked across 7 Voltron decks",
		"avg bracket 3.4",
		"<table class=\"tier-list-table\">",
		"<thead><tr>",
		"Sword of Feast and Famine",
		"Lightning Greaves",
		// Card links resolve to Scryfall via cardLinkHTML.
		"class=\"card-link\"",
		"</table>",
		"</details>",
	}
	for _, s := range musts {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q\n--- output ---\n%s", s, out)
		}
	}
}

// TestRenderHTMLTierList_RendersMetricRows: the table rows include the
// per-card metric columns (inclusion count, rate, avg-bracket-with /
// without, win impact, score).
func TestRenderHTMLTierList_RendersMetricRows(t *testing.T) {
	r := reportForArchetype("Voltron")
	tl := sampleTierList()
	var buf bytes.Buffer
	renderHTMLTierListSection(&buf, r, tl)
	out := buf.String()

	// Inclusion 6/7 → 86%; Sword's row should show 6/7 + 86%.
	if !strings.Contains(out, "6/7") {
		t.Errorf("missing inclusion count 6/7\nout:\n%s", out)
	}
	if !strings.Contains(out, "86%") {
		t.Errorf("missing inclusion rate 86%%\nout:\n%s", out)
	}
	// Signed WinImpact rendered with leading + when positive.
	if !strings.Contains(out, "+1.60") {
		t.Errorf("missing signed +1.60 WinImpact column\nout:\n%s", out)
	}
}

// TestRenderHTMLTierList_NilTierList: nil tier list is a silent no-op.
func TestRenderHTMLTierList_NilTierList(t *testing.T) {
	var buf bytes.Buffer
	renderHTMLTierListSection(&buf, reportForArchetype("Voltron"), nil)
	if buf.Len() != 0 {
		t.Errorf("expected no output for nil tier list, got %d bytes:\n%s",
			buf.Len(), buf.String())
	}
}

// TestRenderHTMLTierList_NoMatchingArchetype: a Voltron deck against a
// tier list that has no Voltron entry renders nothing (corpus didn't
// cover that archetype). Silent skip.
func TestRenderHTMLTierList_NoMatchingArchetype(t *testing.T) {
	r := reportForArchetype("Stax")
	tl := sampleTierList()
	var buf bytes.Buffer
	renderHTMLTierListSection(&buf, r, tl)
	if buf.Len() != 0 {
		t.Errorf("expected no output for unmatched archetype, got:\n%s", buf.String())
	}
}

// TestRenderHTMLTierList_NoArchetype: report without a primary
// archetype → silent skip.
func TestRenderHTMLTierList_NoArchetype(t *testing.T) {
	tests := []*FreyaReport{
		nil,
		{},
		{Profile: nil},
		{Profile: &DeckProfile{PrimaryArchetype: ""}},
	}
	for i, r := range tests {
		var buf bytes.Buffer
		renderHTMLTierListSection(&buf, r, sampleTierList())
		if buf.Len() != 0 {
			t.Errorf("case %d: expected silent skip, got:\n%s", i, buf.String())
		}
	}
}

// TestRenderHTMLTierList_EmptyTopCards: archetype matches but has zero
// top cards → silent skip (no empty table rendered).
func TestRenderHTMLTierList_EmptyTopCards(t *testing.T) {
	r := reportForArchetype("Voltron")
	tl := &TierListExport{
		Archetypes: []ArchetypeTierList{
			{Archetype: "Voltron", DeckCount: 7, TopCards: nil},
		},
	}
	var buf bytes.Buffer
	renderHTMLTierListSection(&buf, r, tl)
	if buf.Len() != 0 {
		t.Errorf("expected silent skip for empty TopCards, got:\n%s", buf.String())
	}
}

// TestLoadTierList_RoundTrip: write a sample TierListExport to disk and
// reload via LoadTierList; verify the in-memory state matches.
func TestLoadTierList_RoundTrip(t *testing.T) {
	prev := LoadedTierList
	t.Cleanup(func() { LoadedTierList = prev })
	LoadedTierList = nil

	dir := t.TempDir()
	path := filepath.Join(dir, "tier_list.json")
	sample := sampleTierList()
	data, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := LoadTierList(path); err != nil {
		t.Fatalf("LoadTierList: %v", err)
	}
	if LoadedTierList == nil {
		t.Fatal("LoadedTierList nil after successful load")
	}
	if LoadedTierList.CorpusSize != sample.CorpusSize {
		t.Errorf("CorpusSize: got %d, want %d", LoadedTierList.CorpusSize, sample.CorpusSize)
	}
	if len(LoadedTierList.Archetypes) != len(sample.Archetypes) {
		t.Errorf("Archetypes len: got %d, want %d",
			len(LoadedTierList.Archetypes), len(sample.Archetypes))
	}
}

// TestLoadTierList_MissingFile: error path — file doesn't exist;
// LoadedTierList stays nil so the renderer falls back to silent skip.
func TestLoadTierList_MissingFile(t *testing.T) {
	prev := LoadedTierList
	t.Cleanup(func() { LoadedTierList = prev })
	LoadedTierList = nil

	err := LoadTierList(filepath.Join(t.TempDir(), "does_not_exist.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if LoadedTierList != nil {
		t.Errorf("LoadedTierList should stay nil on error, got %+v", LoadedTierList)
	}
}

// TestLoadTierList_BadJSON: error path — file exists but is not valid
// JSON; same fall-back behavior.
func TestLoadTierList_BadJSON(t *testing.T) {
	prev := LoadedTierList
	t.Cleanup(func() { LoadedTierList = prev })
	LoadedTierList = nil

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	err := LoadTierList(path)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if LoadedTierList != nil {
		t.Errorf("LoadedTierList should stay nil on parse error, got %+v",
			LoadedTierList)
	}
}

// TestPrintHTML_IncludesTierListWhenLoaded: end-to-end via printHTML —
// when LoadedTierList is set and the report's archetype matches, the
// section is rendered as part of the full document. Restores
// LoadedTierList in cleanup so test ordering doesn't leak state.
func TestPrintHTML_IncludesTierListWhenLoaded(t *testing.T) {
	prev := LoadedTierList
	t.Cleanup(func() { LoadedTierList = prev })
	LoadedTierList = sampleTierList()

	r := &FreyaReport{
		DeckName: "Smoke Test",
		Profile:  &DeckProfile{PrimaryArchetype: "Voltron"},
	}
	var buf bytes.Buffer
	printHTML(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Auto-includes for Voltron") {
		t.Errorf("printHTML missing tier-list section\n--- output (truncated) ---\n%s",
			truncate(out, 600))
	}
}

// TestPrintHTML_OmitsTierListWhenUnloaded: complementary — when
// LoadedTierList is nil, no tier-list section appears in the output.
func TestPrintHTML_OmitsTierListWhenUnloaded(t *testing.T) {
	prev := LoadedTierList
	t.Cleanup(func() { LoadedTierList = prev })
	LoadedTierList = nil

	r := &FreyaReport{
		DeckName: "Smoke Test",
		Profile:  &DeckProfile{PrimaryArchetype: "Voltron"},
	}
	var buf bytes.Buffer
	printHTML(&buf, r)
	if strings.Contains(buf.String(), "Auto-includes for") {
		t.Errorf("printHTML included tier-list section despite nil LoadedTierList")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
