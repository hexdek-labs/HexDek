package deckparser

import (
	"strings"
	"testing"
)

// aetherhub_r60_test.go — Aetherhub export-format support audit.
//
// Aetherhub's "Copy Deck" text export uses three idioms the pre-fix
// parser mishandled:
//
//  1. `About` preamble section — a `Name <DeckName>` (and sometimes
//     `Format Commander` / `Description ...`) block before the real
//     section headers. Pre-fix, `About` matched no section header so
//     subsequent `Name MyDeck` lines fell through the fallback as
//     qty=1 cards and landed in Unresolved noise.
//
//  2. `SB:` line prefix — MTGA / Aetherhub convention for interleaving
//     sideboard lines with mainboard lines instead of using a Sideboard
//     header. Pre-fix, `SB: 1 Negate` parsed as a qty=1 card named
//     "SB: 1 Negate" → Unresolved if not in meta, but silently
//     duplicated into the mainboard if the suffix matched a known card
//     (e.g. `SB: 1 Sol Ring` would double-count Sol Ring).
//
//  3. MTGA-style `1 Card (SET) 123` lines + `Commander` / `Deck` /
//     `Sideboard` section headers — already handled, but pinned here
//     for the integration shape.

func ahMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sol Ring", "Cyclonic Rift",
		"Lightning Bolt", "Negate", "Doubling Season",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_AetherhubFullExport — end-to-end shape of a
// realistic Aetherhub Commander export.
func TestParseDeckReader_AetherhubFullExport(t *testing.T) {
	text := `About
Name My Atraxa Brew
Format Commander
Description Praetors gonna praetor

Commander
1 Atraxa, Praetors' Voice (CMR) 309

Deck
1 Sol Ring (CMR) 250
1 Cyclonic Rift (MM3) 36
1 Doubling Season (BRR) 18

Sideboard
1 Negate (M21) 54
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, ahMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander: want Atraxa, got %v", commanderListNames(td))
	}
	if len(td.Library) != 3 {
		t.Fatalf("library: want 3 cards (Sol Ring, Cyclonic Rift, Doubling Season), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("expected zero unresolved (About preamble should drop, Sideboard should drop), got %v",
			td.Unresolved)
	}
}

// TestParseDeckReader_AetherhubSBLinePrefix — `SB: 1 Card` form must
// drop entirely. The negative-case test pins the silent-duplicate
// regression: pre-fix, `SB: 1 Sol Ring` would have stuffed a second
// Sol Ring into the mainboard.
func TestParseDeckReader_AetherhubSBLinePrefix(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring (CMR) 250
2 Lightning Bolt (M11) 146
SB: 1 Negate (M21) 54
SB: 1 Sol Ring
sb: 1 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, ahMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 3 {
		t.Fatalf("library: want 3 cards (1 Sol Ring + 2 Lightning Bolt), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	// Count Sol Ring — must be exactly 1, NOT 2 (pre-fix bug doubled it
	// because `SB: 1 Sol Ring` parsed as a qty=1 line that matched meta
	// after cleanCardName stripped the SB: prefix... or rather, it
	// landed in Unresolved as "SB: 1 Sol Ring" — but a future regex
	// tweak to cleanCardName could surface the latent duplicate).
	solRing := 0
	bolt := 0
	for _, c := range td.Library {
		switch c.Name {
		case "Sol Ring":
			solRing++
		case "Lightning Bolt":
			bolt++
		}
	}
	if solRing != 1 {
		t.Errorf("Sol Ring count: want 1, got %d — SB: prefix leaked into mainboard", solRing)
	}
	if bolt != 2 {
		t.Errorf("Lightning Bolt count: want 2 (mainboard only, SB lowercase variant should also drop), got %d", bolt)
	}
	for _, u := range td.Unresolved {
		if strings.HasPrefix(strings.ToUpper(u), "SB:") {
			t.Errorf("SB: prefix leaked into Unresolved: %q", u)
		}
	}
}

// TestParseDeckReader_AetherhubAboutPreamble — the About / Name / Format
// / Description headers must not become Unresolved card names.
func TestParseDeckReader_AetherhubAboutPreamble(t *testing.T) {
	text := `About
Name Some Deck Name With Several Words
Format Commander
Description This deck does the thing.
Author someuser

Commander
1 Atraxa, Praetors' Voice

Deck
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, ahMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("About preamble should drop entirely, got Unresolved=%v", td.Unresolved)
	}
	if len(td.Library) != 1 || td.Library[0].Name != "Sol Ring" {
		t.Errorf("library: want [Sol Ring], got %v", libraryNames(td.Library))
	}
}

// TestParseDeckReader_AetherhubNegativeOfFix_NormalDeckUnchanged — a
// Moxfield-shape deck (no Aetherhub idioms) parses identically before
// and after this fix. Defends against the section-drop / SB-strip code
// path swallowing real lines.
func TestParseDeckReader_AetherhubNegativeOfFix_NormalDeckUnchanged(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Cyclonic Rift
2 Lightning Bolt
1 Doubling Season
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, ahMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 5 {
		t.Errorf("library: want 5 (1+1+2+1), got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("expected zero unresolved on plain deck, got %v", td.Unresolved)
	}
}

func commanderListNames(td *TournamentDeck) []string {
	var out []string
	for _, c := range td.CommanderCards {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}
