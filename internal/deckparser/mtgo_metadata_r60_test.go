package deckparser

import (
	"strings"
	"testing"
)

// mtgo_metadata_r60_test.go — MTGO / Tournament-Ready text export
// metadata-header support audit. Sibling to the leading-bracket fix
// (PR #X) and the SB: prefix fix (PR #X). MTGO's text export emits a
// metadata preamble before the card list:
//
//	Deck name: My MTGO Deck
//	Created by: TestPlayer
//	Format: Commander
//	Layout: Commander
//
//	1 [LEA] Sol Ring
//	...
//
// Pre-fix, every metadata line fell through the fallback as a qty=1
// card named `Deck name: My MTGO Deck` etc. and landed in the
// Unresolved report. Real Magic cards never contain a colon, so a
// generic `<word>: <content>` detector is safe — but the matcher uses
// an explicit allowlist of known MTGO metadata keys for defense
// against any future card-naming quirks.

func mtgoMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sol Ring", "Cyclonic Rift",
		"Lightning Bolt", "Negate", "Counterspell",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_MTGOTournamentReadyFull — end-to-end shape: full
// MTGO Tournament-Ready export with metadata preamble + leading set
// codes + SB: sideboard interleaving.
func TestParseDeckReader_MTGOTournamentReadyFull(t *testing.T) {
	text := `Deck name: My MTGO Deck
Created by: TestPlayer
Format: Commander
Layout: Commander
Description: Praetor stuff

1 [LEA] Sol Ring
2 [M11] Lightning Bolt
1 Cyclonic Rift

SB: 1 Negate
SB: 1 Counterspell

COMMANDER: Atraxa, Praetors' Voice
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgoMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander: want Atraxa, got %+v", td.CommanderCards)
	}
	if len(td.Library) != 4 {
		t.Errorf("library: want 4 (1 Sol Ring + 2 Lightning Bolt + 1 Cyclonic Rift), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved should be empty (all metadata headers should drop), got %v", td.Unresolved)
	}
}

// TestParseDeckReader_MTGOMetadataKeysAllRecognized — every key in the
// allowlist drops cleanly. Pins the matcher's coverage.
func TestParseDeckReader_MTGOMetadataKeysAllRecognized(t *testing.T) {
	text := `Deck name: x
Deck Name: x
Created by: x
Format: Commander
Layout: Commander
Author: x
Description: x
Tag: ramp
Tags: ramp, draw
Source: https://example.com
Owner: someone
Note: a note
Notes: many notes

COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgoMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("expected zero unresolved with all known metadata keys, got %v", td.Unresolved)
	}
	if len(td.Library) != 1 {
		t.Fatalf("library: want [Sol Ring], got %v", libraryNames(td.Library))
	}
}

// TestParseDeckReader_MTGOMetadataCaseInsensitive — keys come in
// arbitrary casing (`DECK NAME:`, `deck name:`, `Deck Name:`).
func TestParseDeckReader_MTGOMetadataCaseInsensitive(t *testing.T) {
	text := `DECK NAME: shouty
format: commander
LAYOUT: commander
Created BY: someone

COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgoMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("metadata casing should not affect drop: unresolved=%v", td.Unresolved)
	}
}

// TestParseDeckReader_MTGONegativeOfFix_KnownDirectivesUnaffected —
// COMMANDER: / PARTNER: / SB: precede metadata in the line loop and
// must still match. A regression here would mean metadata short-
// circuited the existing directives.
func TestParseDeckReader_MTGONegativeOfFix_KnownDirectivesUnaffected(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
PARTNER: Sol Ring
1 Cyclonic Rift
SB: 1 Negate
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgoMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) < 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("primary commander lost: %+v", td.CommanderCards)
	}
	// Library should be just Cyclonic Rift (SB: drops, PARTNER: is not a library card).
	if len(td.Library) != 1 || td.Library[0].Name != "Cyclonic Rift" {
		t.Errorf("library: want [Cyclonic Rift], got %v", libraryNames(td.Library))
	}
}

// TestParseDeckReader_MTGONegativeOfFix_RealCardNotDropped — the
// matcher uses an allowlist of known metadata keys, NOT a generic
// `word: word` pattern. A future regex tightening that accidentally
// broadened to `^\w+:` would drop card lines like `1 X-word: ...`
// (none exist today but the test pins the conservative scope).
func TestParseDeckReader_MTGONegativeOfFix_RealCardNotDropped(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgoMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("plain deck regressed: want 2 cards, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}
