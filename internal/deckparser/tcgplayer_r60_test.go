package deckparser

import (
	"strings"
	"testing"
)

// tcgplayer_r60_test.go — TCGplayer mass-entry format audit.
//
// TCGplayer's mass-entry tool at /massentry accepts:
//
//	{qty} {card name} [{set code or full set name}]
//	{qty} {card name} [{set code or full set name}] {collector number}
//
// The first form was already handled by the trailing-bracket strip
// (bracketTagRE). The second — `[SET] 146`, `[Modern Masters 2017] 36`,
// `[Commander 2014] ★146` — was NOT, because the bracket isn't at
// end-of-line. Pre-fix, every TCGplayer line with a collector tail
// landed in Unresolved despite the meta lookup being trivially
// possible after the strip.
//
// Fix extends bracketTagRE to allow optional `\s+\d+\S*` trailing
// collector number after the bracket. The `\d+\S*` head requires a
// digit, so a hypothetical card-name suffix like `[Token Pack] Goblin`
// (no digit) would NOT match.

func tcgMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sol Ring", "Cyclonic Rift",
		"Lightning Bolt", "Goblin Guide", "Lim-Dûl's Vault",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_TCGplayerMassEntryWithCollectorNumber — the
// load-bearing fix: trailing `[SET] COLLECTOR` form parses cleanly.
func TestParseDeckReader_TCGplayerMassEntryWithCollectorNumber(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lightning Bolt [M11] 146
1 Cyclonic Rift [Modern Masters 2017] 36
4 Sol Ring [Commander 2014] 263
1 Goblin Guide [Zendikar] ★114
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, tcgMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 7 {
		t.Fatalf("library: want 7 (1+1+4+1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("expected zero unresolved (TCGplayer collector form), got %v", td.Unresolved)
	}
}

// TestParseDeckReader_TCGplayerMassEntryBracketOnly — the form that
// already worked, pinned to defend against regex tightening.
func TestParseDeckReader_TCGplayerMassEntryBracketOnly(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lightning Bolt [M11]
4 Sol Ring [Commander 2014]
1 Cyclonic Rift [Modern Masters 2017]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, tcgMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 6 {
		t.Errorf("library: want 6 (1+4+1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestCleanCardName_TCGplayerBracketCollectorPairs — helper-level pin
// so a regex rewrite that re-breaks the collector-tail strip stays
// caught even when the full-parse integration test passes for other
// reasons.
func TestCleanCardName_TCGplayerBracketCollectorPairs(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Lightning Bolt [M11] 146", "Lightning Bolt"},
		{"Sol Ring [Commander 2014] 263", "Sol Ring"},
		{"Cyclonic Rift [Modern Masters 2017] 36", "Cyclonic Rift"},
		{"Goblin Guide [Zendikar] ★114", "Goblin Guide"},
		// Bracket-only (no collector) still works.
		{"Lightning Bolt [M11]", "Lightning Bolt"},
		{"Sol Ring [Commander 2014]", "Sol Ring"},
		// No decoration at all.
		{"Lightning Bolt", "Lightning Bolt"},
		// Hyphen in card name must NOT confuse the strip.
		{"Lim-Dûl's Vault", "Lim-Dûl's Vault"},
		{"Lim-Dûl's Vault [Alliances]", "Lim-Dûl's Vault"},
		{"Lim-Dûl's Vault [Alliances] 60", "Lim-Dûl's Vault"},
	}
	for _, tc := range cases {
		got := cleanCardName(tc.in)
		if got != tc.want {
			t.Errorf("cleanCardName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCleanCardName_TCGplayerNegativeOfFix_NoCardNamesHaveBrackets —
// the `\S+` collector token after the bracket is permissive (covers ★
// prefixes etc.) but safe because no real Magic card name contains
// `[...]`. Anything after a bracket in a parser-fed line is by
// definition decoration. This test pins that real card names with
// punctuation that LOOKS like decoration (hyphens, apostrophes,
// accented chars) are not corrupted.
func TestCleanCardName_TCGplayerNegativeOfFix_NoCardNamesHaveBrackets(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Card names with hyphens / apostrophes / accents are untouched
		// because they have no bracket.
		{"Lim-Dûl's Vault", "Lim-Dûl's Vault"},
		{"Spike-Tailed Ceratops", "Spike-Tailed Ceratops"},
		{"Jack-in-the-Mox", "Jack-in-the-Mox"},
		{"Lim-Dûl's Vault [Alliances] 60", "Lim-Dûl's Vault"},
	}
	for _, tc := range cases {
		got := cleanCardName(tc.in)
		if got != tc.want {
			t.Errorf("cleanCardName(%q) = %q, want %q (real card name corrupted)",
				tc.in, got, tc.want)
		}
	}
}

// TestParseDeckReader_TCGplayerMixedExport — realistic mass-entry
// paste with a mix of bracket-only and bracket+collector lines (the
// shape TCGplayer's "Export Cart as Text" actually emits).
func TestParseDeckReader_TCGplayerMixedExport(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring [Commander 2014]
1 Lightning Bolt [M11] 146
4 Cyclonic Rift [Modern Masters 2017] 36
1 Goblin Guide [Zendikar]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, tcgMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 7 {
		t.Errorf("library: want 7 (1+1+4+1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("mixed TCGplayer export should leave no unresolved, got %v", td.Unresolved)
	}
}
