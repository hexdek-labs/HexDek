package deckparser

import (
	"strings"
	"testing"
)

// TestParseDeckReader_MoxfieldNativeSectionCounts is the worst-gap regression
// surfaced by the r60 edge-case audit.
//
// Moxfield's native "Plain Text" export (the most-used export mode for
// pasting into a chat or text file) emits section headers with a trailing
// parenthesized count: `Commander (1)`, `Deck (99)`, `Sideboard (5)`,
// `Companion (1)`. The pre-fix `sectionHeaderRE` required the line to end
// with `:` or whitespace and did NOT tolerate the trailing `(N)` count, so
// every Moxfield-native paste:
//
//  1. Slipped through the section-header match (section stays "main").
//  2. Hit the catch-all set-paren strip `if idx := strings.Index(raw, "(")`,
//     reducing "Sideboard (5)" to the literal word "Sideboard".
//  3. Treated "Sideboard" as a card name → added to Unresolved.
//  4. Continued parsing subsequent lines in "main" section, so every
//     sideboard / companion / token card silently leaked into the library.
//
// This is silent corruption — the deck "parses" but the library is wrong.
// The first symptom a user sees is unexpected cards drawn in-game.
func TestParseDeckReader_MoxfieldNativeSectionCounts(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice",
		"Sol Ring",
		"Command Tower",
		"Lurrus of the Dream-Den",
		"Hangarback Walker",
		"Treasure Token",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 3,
		}
	}
	// Verbatim shape of Moxfield's native plaintext export.
	text := `Commander (1)
1 Atraxa, Praetors' Voice

Deck (99)
1 Sol Ring
1 Command Tower

Companion (1)
1 Lurrus of the Dream-Den

Sideboard (1)
1 Hangarback Walker

Tokens (1)
1 Treasure Token
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("ParseDeckReader: %v", err)
	}
	if td.CommanderName != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander want Atraxa, got %q", td.CommanderName)
	}
	// Library must be exactly the 2 Deck-section cards — no sideboard /
	// companion / token leak.
	if len(td.Library) != 2 {
		t.Fatalf("library want 2 (Deck section only), got %d: %v",
			len(td.Library), libraryNames(td.Library))
	}
	for _, c := range td.Library {
		switch c.Name {
		case "Lurrus of the Dream-Den":
			t.Errorf("companion %q leaked into library", c.Name)
		case "Hangarback Walker":
			t.Errorf("sideboard %q leaked into library", c.Name)
		case "Treasure Token":
			t.Errorf("token %q leaked into library", c.Name)
		}
	}
	// And no spurious "Sideboard"/"Companion"/"Tokens" string-as-card
	// entries in Unresolved.
	for _, u := range td.Unresolved {
		switch u {
		case "Sideboard", "Companion", "Tokens", "Deck", "Commander":
			t.Errorf("section header %q leaked into Unresolved as a phantom card name", u)
		}
	}
}

// TestParseDeckReader_MoxfieldNativePartnerCount covers the partner-pair
// case under a `Commanders (2)` header (Moxfield's actual export for a
// partner deck). The pre-fix regex didn't match this header so both
// partner commanders ended up in the library and the FIRST resolvable
// line became the (single) commander — silent re-shaping of the partner
// pair into a solo deck.
func TestParseDeckReader_MoxfieldNativePartnerCount(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Kraum, Ludevic's Opus",
		"Tymna the Weaver",
		"Sol Ring",
		"Command Tower",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 2,
		}
	}
	text := `Commanders (2)
1 Kraum, Ludevic's Opus
1 Tymna the Weaver

Deck (98)
1 Sol Ring
1 Command Tower
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("ParseDeckReader: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("want 2 commanders, got %d: %v",
			len(td.CommanderCards), td.CommanderNames())
	}
	names := td.CommanderNames()
	if names[0] != "Kraum, Ludevic's Opus" || names[1] != "Tymna the Weaver" {
		t.Errorf("commander order wrong: %v", names)
	}
	if len(td.Library) != 2 {
		t.Fatalf("library want 2, got %d: %v",
			len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_SectionCountWithColon covers a less common but valid
// Moxfield/Archidekt variant: `Sideboard: (5)` or `Sideboard (5):`. Both
// shapes occur in hand-edited lists and the parser should treat them
// uniformly.
func TestParseDeckReader_SectionCountWithColon(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Hangarback Walker"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
1 Sol Ring

Sideboard: (1)
1 Hangarback Walker
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 1 {
		t.Fatalf("library want 1, got %d: %v",
			len(td.Library), libraryNames(td.Library))
	}
	for _, c := range td.Library {
		if c.Name == "Hangarback Walker" {
			t.Error("sideboard card leaked into library")
		}
	}
}
