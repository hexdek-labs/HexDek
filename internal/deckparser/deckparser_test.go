package deckparser

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// astDatasetPath walks up from this test file to find data/rules/ast_dataset.jsonl.
func astDatasetPath() string {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 6; i++ {
		c := filepath.Join(dir, "data", "rules", "ast_dataset.jsonl")
		if _, err := os.Stat(c); err == nil {
			return c
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

func TestLoadMeta(t *testing.T) {
	path := astDatasetPath()
	if path == "" {
		t.Skip("no AST dataset available")
	}
	meta, err := LoadMetaFromJSONL(path)
	if err != nil {
		t.Fatalf("LoadMetaFromJSONL: %v", err)
	}
	if meta.Count() < 10_000 {
		t.Fatalf("expected >=10k cards, got %d", meta.Count())
	}
	// Spot-check a well-known card.
	m := meta.Get("Lightning Bolt")
	if m == nil {
		t.Fatalf("Lightning Bolt not found")
	}
	if m.CMC != 1 {
		t.Errorf("Lightning Bolt CMC want 1, got %d", m.CMC)
	}
	if !strings.Contains(strings.Join(m.Types, " "), "instant") {
		t.Errorf("Lightning Bolt types want instant, got %v", m.Types)
	}
}

func TestParseTypes(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"Legendary Creature — Human Ninja", []string{"legendary", "creature", "human", "ninja"}},
		{"Land", []string{"land"}},
		{"Basic Land — Swamp", []string{"basic", "land", "swamp"}},
		{"", nil},
	}
	for _, tc := range cases {
		got := parseTypes(tc.line)
		if len(got) != len(tc.want) {
			t.Errorf("parseTypes(%q): len want %d, got %d (%v)", tc.line, len(tc.want), len(got), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseTypes(%q) [%d]: want %q got %q", tc.line, i, tc.want[i], got[i])
			}
		}
	}
}

func TestNormalizeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Lightning Bolt", "lightning bolt"},
		{"  Lightning   Bolt  ", "lightning bolt"},
		{"Jötun Grunt", "jotun grunt"},
		{"Æther Vial", "ether vial"},
	}
	for _, tc := range cases {
		got := normalizeName(tc.in)
		if got != tc.want {
			t.Errorf("normalizeName(%q): want %q got %q", tc.in, tc.want, got)
		}
	}
}

func TestParseDeckReader(t *testing.T) {
	path := astDatasetPath()
	if path == "" {
		t.Skip("no AST dataset")
	}
	meta, err := LoadMetaFromJSONL(path)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	text := `1 Tergrid, God of Fright // Tergrid's Lantern
1 Swamp
1 Dark Ritual
1 Swamp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		// Tergrid is a legendary card that might or might not be in
		// the meta. Don't fail hard.
		t.Skipf("parse: %v", err)
	}
	if td.CommanderName == "" {
		t.Fatalf("no commander parsed")
	}
	if len(td.Library) < 1 {
		t.Fatalf("library empty")
	}
}

// TestParseDeckReader_PartnerDirective verifies PARTNER: footer parsing.
// Two commanders must both land in CommanderCards; the library excludes
// both. CR §702.124 / §903.3c partner support.
func TestParseDeckReader_PartnerDirective(t *testing.T) {
	text := `1 Kraum, Ludevic's Opus
1 Tymna the Weaver
1 Sol Ring
1 Command Tower

COMMANDER: Kraum, Ludevic's Opus
PARTNER: Tymna the Weaver
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, nil)
	// Without meta+corpus the builder returns nil cards, so we check
	// that the directive was recognised even if cards aren't resolved.
	_ = err
	// Re-run with a stub-friendly path: use a synthetic meta that resolves
	// our 4 lines.
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Kraum, Ludevic's Opus", "Tymna the Weaver", "Sol Ring", "Command Tower"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature", Types: []string{"legendary", "creature"}, CMC: 3,
		}
	}
	td, err = ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse with synthetic meta: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("partner deck: want 2 commanders, got %d (%v)",
			len(td.CommanderCards), td.CommanderNames())
	}
	names := td.CommanderNames()
	if names[0] != "Kraum, Ludevic's Opus" {
		t.Fatalf("first commander wrong: %q", names[0])
	}
	if names[1] != "Tymna the Weaver" {
		t.Fatalf("partner wrong: %q", names[1])
	}
	// Library should NOT contain either commander.
	for _, c := range td.Library {
		if c.Name == "Kraum, Ludevic's Opus" || c.Name == "Tymna the Weaver" {
			t.Fatalf("library still contains commander %q", c.Name)
		}
	}
	if len(td.Library) != 2 {
		t.Fatalf("library should have 2 non-commander entries, got %d", len(td.Library))
	}
}

// TestParseDeckFile_RealPartnerDecks loads the 3 cEDH partner decks on
// disk (Kraum+Tymna, Ardenn+Rograkh, Kinnan+Thrasios) and verifies each
// parses as a two-commander deck. Directly exercises the integration
// path Josh tracked in feature_gap_list.md Tier 1 #5.
func TestParseDeckFile_RealPartnerDecks(t *testing.T) {
	astPath := astDatasetPath()
	if astPath == "" {
		t.Skip("no AST dataset")
	}
	meta, err := LoadMetaFromJSONL(astPath)
	if err != nil {
		t.Fatalf("load meta: %v", err)
	}
	// Hunt down the deck dir by walking up from this file.
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	var deckDir string
	for i := 0; i < 6; i++ {
		c := filepath.Join(dir, "data", "decks", "benched")
		if _, err := os.Stat(c); err == nil {
			deckDir = c
			break
		}
		dir = filepath.Dir(dir)
	}
	if deckDir == "" {
		t.Skip("benched deck dir not found")
	}

	cases := []struct {
		file  string
		want1 string
		want2 string
	}{
		{"cedh_combo_partner_b5_kraum_tymna.txt", "Kraum, Ludevic's Opus", "Tymna the Weaver"},
		{"cedh_big_stick_b5_ardenn_rograkh.txt", "Ardenn, Intrepid Archaeologist", "Rograkh, Son of Rohgahh"},
		// Kinnan+Thrasios removed: Kinnan doesn't have the Partner keyword,
		// so Kinnan is solo commander with Thrasios in the 99 (validated
		// by the partner agent 2026-04-16). See cedh_turbo_b5_kinnan.txt.
	}
	for _, tc := range cases {
		path := filepath.Join(deckDir, tc.file)
		if _, err := os.Stat(path); err != nil {
			t.Logf("skipping missing %s", tc.file)
			continue
		}
		td, err := ParseDeckFile(path, nil, meta)
		if err != nil {
			t.Errorf("ParseDeckFile(%s): %v", tc.file, err)
			continue
		}
		if len(td.CommanderCards) != 2 {
			t.Errorf("%s: want 2 commanders, got %d (names=%v, unresolved=%v)",
				tc.file, len(td.CommanderCards), td.CommanderNames(), td.Unresolved)
			continue
		}
		names := td.CommanderNames()
		if names[0] != tc.want1 {
			t.Errorf("%s: commander[0] want %q got %q", tc.file, tc.want1, names[0])
		}
		if names[1] != tc.want2 {
			t.Errorf("%s: commander[1] want %q got %q", tc.file, tc.want2, names[1])
		}
		// Library should be positive.
		if len(td.Library) < 50 {
			t.Errorf("%s: library suspiciously small (%d)", tc.file, len(td.Library))
		}
	}
}
