package deckparser

import (
	"strings"
	"testing"
)

// commander_setcode_r64_test.go — two parser defects filed 2026-09-29 and
// 2026-10-09, both of the "the parser needs a marker the user's exporter
// never writes" family.
//
//  1. The `(SET) 123` printing tail was stripped only by the quantity-line
//     path, so a Moxfield-style `COMMANDER: Maarika, Brutal Gladiator
//     (SLX) 15` resolved to no commander and the deck became unplayable in
//     the engine (HTTP 404 from the gauntlet). Reported live by a user whose
//     four decks all carried it.
//  2. Comment-style section markers (`//maybeboard`, `// Sideboard`,
//     `// COMMANDER`) were eaten by the blanket comment-skip before the
//     section-header check ran, so maybeboard cards counted as main deck
//     (100 + 38 = 138 -> false ILLEGAL).

func setcodeMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Maarika, Brutal Gladiator", "Tatyova, Benthic Druid",
		"Kelsien, the Plague", "Thrasios, Triton Hero", "Vial Smasher the Fierce",
		"Sol Ring", "Lightning Bolt", "Forest", "Mountain", "Swamp",
		"Hazmat Suit (Used)",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 2,
		}
	}
	return meta
}

func mustParse(t *testing.T, text string) *TournamentDeck {
	t.Helper()
	td, err := ParseDeckReader(strings.NewReader(text), nil, setcodeMeta())
	if err != nil {
		t.Fatalf("ParseDeckReader returned error: %v", err)
	}
	return td
}

func r64CommanderNames(td *TournamentDeck) []string {
	out := make([]string, 0, len(td.CommanderCards))
	for _, c := range td.CommanderCards {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}

// TestCommanderDirective_SetCollectorTailStripped is the exact live repro:
// pre-fix this returned `deckparser: no commander found
// (directive="Maarika, Brutal Gladiator (SLX) 15")`.
func TestCommanderDirective_SetCollectorTailStripped(t *testing.T) {
	td := mustParse(t, `1 Sol Ring (LTC) 294
1 Lightning Bolt (2X2) 117
COMMANDER: Maarika, Brutal Gladiator (SLX) 15
`)
	got := r64CommanderNames(td)
	if len(got) != 1 || got[0] != "Maarika, Brutal Gladiator" {
		t.Fatalf("commander = %v, want [Maarika, Brutal Gladiator]", got)
	}
}

// TestPartnerDirective_SetCollectorTailStripped — the PARTNER: path calls
// cleanCardName the same way and had the same hole.
func TestPartnerDirective_SetCollectorTailStripped(t *testing.T) {
	td := mustParse(t, `1 Sol Ring
COMMANDER: Thrasios, Triton Hero (C16) 38
PARTNER: Vial Smasher the Fierce (C16) 40
`)
	got := r64CommanderNames(td)
	if len(got) != 2 || got[0] != "Thrasios, Triton Hero" || got[1] != "Vial Smasher the Fierce" {
		t.Fatalf("commanders = %v, want [Thrasios, Triton Hero Vial Smasher the Fierce]", got)
	}
}

// TestCommanderDirective_PromoCollectorSuffix — collector numbers carry
// variant letters and promo glyphs in real exports.
func TestCommanderDirective_PromoCollectorSuffix(t *testing.T) {
	for _, line := range []string{
		"COMMANDER: Kelsien, the Plague (C20) 11a",
		"COMMANDER: Kelsien, the Plague (C20) 11★",
	} {
		td := mustParse(t, "1 Sol Ring\n"+line+"\n")
		if got := r64CommanderNames(td); len(got) != 1 || got[0] != "Kelsien, the Plague" {
			t.Fatalf("line %q -> commander %v, want [Kelsien, the Plague]", line, got)
		}
	}
}

// TestCleanCardName_KeepsParentheticalCardName is the conservatism guard:
// the strip requires a collector number precisely so that real card names
// ending in parentheses survive. If this ever fails, the regex has been
// loosened into eating legitimate names.
func TestCleanCardName_KeepsParentheticalCardName(t *testing.T) {
	if got := cleanCardName("Hazmat Suit (Used)"); got != "Hazmat Suit (Used)" {
		t.Fatalf("cleanCardName(%q) = %q, want it unchanged", "Hazmat Suit (Used)", got)
	}
}

// TestCleanCardName_SetTailWithOtherDecoration — the strip lives inside the
// repeat-peel loop, so it composes with foil markers and hash tags.
func TestCleanCardName_SetTailWithOtherDecoration(t *testing.T) {
	for in, want := range map[string]string{
		"Sol Ring (LEA) 270":       "Sol Ring",
		"Sol Ring (LEA) 270 *F*":   "Sol Ring",
		"Sol Ring (LEA) 270 #ramp": "Sol Ring",
		"Sol Ring":                 "Sol Ring",
		"Sol Ring (LEA)":           "Sol Ring (LEA)", // bare set, no number: left alone
	} {
		if got := cleanCardName(in); got != want {
			t.Errorf("cleanCardName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCommentStyleMaybeboard_NotCountedAsMain — the 138-card repro.
func TestCommentStyleMaybeboard_NotCountedAsMain(t *testing.T) {
	td := mustParse(t, `COMMANDER: Maarika, Brutal Gladiator (SLX) 15
1 Sol Ring
1 Lightning Bolt
//maybeboard
1 Forest
1 Mountain
1 Swamp
`)
	for _, c := range td.Library {
		if c != nil && (c.Name == "Forest" || c.Name == "Mountain" || c.Name == "Swamp") {
			t.Fatalf("maybeboard card %q leaked into the main deck (library=%d)", c.Name, len(td.Library))
		}
	}
}

// TestCommentStyleSectionMarkers_Variants — spacing and casing vary by
// exporter; all of these are section markers, not comments.
func TestCommentStyleSectionMarkers_Variants(t *testing.T) {
	for _, marker := range []string{"//maybeboard", "// Maybeboard", "//Considering", "// Sideboard", "#Sideboard"} {
		td := mustParse(t, "COMMANDER: Maarika, Brutal Gladiator (SLX) 15\n1 Sol Ring\n"+marker+"\n1 Forest\n")
		for _, c := range td.Library {
			if c != nil && c.Name == "Forest" {
				t.Errorf("marker %q: Forest leaked into main deck", marker)
			}
		}
	}
}

// TestProseComment_StillDroppedNotTreatedAsSection — the guard on the other
// side: an ordinary comment must not be mistaken for a section header.
func TestProseComment_StillDroppedNotTreatedAsSection(t *testing.T) {
	td := mustParse(t, `COMMANDER: Maarika, Brutal Gladiator (SLX) 15
1 Sol Ring
// Deck built for casual play, sideboard ideas below
1 Lightning Bolt
`)
	var sawBolt bool
	for _, c := range td.Library {
		if c != nil && c.Name == "Lightning Bolt" {
			sawBolt = true
		}
	}
	if !sawBolt {
		t.Fatal("Lightning Bolt after a prose comment was dropped — the comment was misread as a section header")
	}
}

// TestCommentStyleCommanderHeader — the Moxfield `// COMMANDER` export
// shape, third member of the same family.
func TestCommentStyleCommanderHeader(t *testing.T) {
	td := mustParse(t, `// COMMANDER
1 Maarika, Brutal Gladiator (SLX) 15
// Deck
1 Sol Ring
`)
	if got := r64CommanderNames(td); len(got) != 1 || got[0] != "Maarika, Brutal Gladiator" {
		t.Fatalf("commander = %v, want [Maarika, Brutal Gladiator]", got)
	}
}
