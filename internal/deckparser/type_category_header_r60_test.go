package deckparser

import (
	"strings"
	"testing"
)

// type_category_header_r60_test.go — Moxfield "Card View" / Archidekt
// "By Type" sub-section header support.
//
// Both Moxfield (default "Card View" copy-to-clipboard) and Archidekt
// (default "By Type" export) emit type-line categorization headers
// inside the mainboard:
//
//	Commander
//	1 Atraxa, Praetors' Voice
//
//	Creatures (24)
//	1 Birds of Paradise
//	...
//	Planeswalkers (3)
//	1 Liliana of the Veil
//	...
//	Lands (38)
//	1 Forest
//	...
//
// Pre-fix `Creatures (24)` etc. weren't in `sectionHeaderRE`'s alternate-
// board whitelist (Sideboard/Maybeboard/Companion/etc.), so they fell
// through the fallback as qty=1 fake cards. WORSE: because the line
// didn't transition section state, the parser stayed in the "commander"
// section it had entered for the `Commander` header above — so every
// subsequent creature/spell line routed to commander slots. The second
// real card (after Atraxa) became a bogus partner candidate, then 96+
// more cards landed in commanderSectionNames (silently truncated to the
// partner slot since CR §903.5b caps at 2). Net effect: Library = 0,
// Unresolved = ["Creatures"], deck completely broken — a much worse
// outcome than the user would notice without tracing through td.Library.
//
// Fix adds typeCategoryHeaderRE matching the canonical type names
// (Creatures, Planeswalkers, Battles, Sorceries, Instants, Artifacts,
// Enchantments, Lands, Tribal, Other, Spells — plurals primary, with
// singular variants for hand-edits) with optional `(N)` count. Matched
// lines drop; section transitions from "commander" to "main" (the
// headers reliably mark the end of the commander block in real
// exports). Stays in "drop" if currently dropped (sideboards can have
// their own type buckets and we don't want to re-promote into main).

func typeCategoryMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Birds of Paradise", "Wood Elves",
		"Solemn Simulacrum", "Liliana of the Veil", "Demonic Tutor",
		"Cultivate", "Counterspell", "Lightning Bolt", "Sol Ring",
		"Sylvan Library", "Forest", "Swamp", "Plains",
		"Spirited Companion", // a real card whose name overlaps the "Companion" alt-board label
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Creature", Types: []string{"creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_MoxfieldCardViewFullExport — the corpus-killer
// scenario. A complete Moxfield "Card View" copy-paste with 8 type-
// category headers (Commander / Creatures / Planeswalkers / Sorceries /
// Instants / Artifacts / Enchantments / Lands) all resolve to the
// correct sections: 1 commander, 13 mainboard cards, 0 unresolved.
// Pre-fix this produced Library=0, Unresolved=["Creatures"].
func TestParseDeckReader_MoxfieldCardViewFullExport(t *testing.T) {
	text := `Commander (1)
1 Atraxa, Praetors' Voice

Creatures (3)
1 Birds of Paradise
1 Wood Elves
1 Solemn Simulacrum

Planeswalkers (1)
1 Liliana of the Veil

Sorceries (2)
1 Demonic Tutor
1 Cultivate

Instants (2)
1 Counterspell
1 Lightning Bolt

Artifacts (1)
1 Sol Ring

Enchantments (1)
1 Sylvan Library

Lands (3)
1 Forest
1 Swamp
1 Plains
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander: got %v, want [Atraxa, Praetors' Voice]",
			commanderNames(td))
	}
	if len(td.Library) != 13 {
		t.Errorf("Library: want 13 mainboard cards, got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none (every category header should drop, not Unresolve), got %v", td.Unresolved)
	}
	// Spot-check that NO CardLine carries a category header as a card name.
	for _, cl := range td.CardLines {
		switch strings.ToLower(cl.Name) {
		case "creatures", "planeswalkers", "sorceries", "instants",
			"artifacts", "enchantments", "lands":
			t.Errorf("category header leaked into CardLines as card: %+v", cl)
		}
	}
}

// TestParseDeckReader_CategoryHeadersWithoutCount — bare `Lands` /
// `Creatures` without `(N)` suffix (hand-edited TappedOut-style exports
// often omit the count). The regex's count group is optional.
func TestParseDeckReader_CategoryHeadersWithoutCount(t *testing.T) {
	text := `Commander
1 Atraxa, Praetors' Voice

Creatures
1 Birds of Paradise
1 Wood Elves

Lands
1 Forest
1 Swamp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 4 {
		t.Errorf("Library: want 4 (Birds, Wood Elves, Forest, Swamp), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: %v", td.Unresolved)
	}
}

// TestParseDeckReader_CategoryHeadersAllCanonicalLabels — every label
// the regex covers must drop and transition state correctly. Single
// representative card under each header to verify the transition.
func TestParseDeckReader_CategoryHeadersAllCanonicalLabels(t *testing.T) {
	text := `Commander
1 Atraxa, Praetors' Voice

Creatures (1)
1 Birds of Paradise
Planeswalkers (1)
1 Liliana of the Veil
Battles (0)
Sorceries (1)
1 Demonic Tutor
Sorcery
Instants (1)
1 Counterspell
Instant
Artifacts (1)
1 Sol Ring
Artifact
Enchantments (1)
1 Sylvan Library
Enchantment
Lands (1)
1 Forest
Land
Tribal (0)
Other (0)
Spells (0)
Spell
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 7 {
		t.Errorf("Library: want 7 (one per populated category), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none (every label should match the regex), got %v", td.Unresolved)
	}
}

// TestParseDeckReader_CategoryHeadersDontPromoteFromDrop — when a
// category header appears INSIDE a sideboard/maybeboard, we must NOT
// promote subsequent cards back to mainboard. Sideboards can have
// their own type buckets (Aetherhub-style "Sideboard Creatures (5)")
// and re-promoting would silently inflate the mainboard.
func TestParseDeckReader_CategoryHeadersDontPromoteFromDrop(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring

Sideboard (3)
Creatures (2)
1 Birds of Paradise
1 Wood Elves
Lands (1)
1 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Only Sol Ring should be in the mainboard — everything under
	// `Sideboard (3)` is in the drop section, and the type headers
	// nested inside it must NOT re-promote those cards.
	if len(td.Library) != 1 || td.Library[0].Name != "Sol Ring" {
		t.Errorf("Library: want [Sol Ring] only (sideboard cards must stay dropped), got %v",
			libraryNames(td.Library))
	}
}

// TestParseDeckReader_CommanderToMainTransition — the section-state
// transition. After `Commander` + `1 Atraxa`, encountering `Creatures
// (N)` must shift section to "main" so subsequent cards route to the
// library, not to commander slots. This is the load-bearing fix.
func TestParseDeckReader_CommanderToMainTransition(t *testing.T) {
	text := `Commander (1)
1 Atraxa, Praetors' Voice

Creatures (2)
1 Birds of Paradise
1 Wood Elves
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Pre-fix the parser stayed in "commander" section forever, so
	// Birds of Paradise + Wood Elves ended up as commander/partner
	// candidates. Library was empty.
	if len(td.CommanderCards) != 1 {
		t.Errorf("CommanderCards: want 1 (just Atraxa), got %d (%v) — type-category header didn't transition out of commander",
			len(td.CommanderCards), commanderNames(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("Library: want 2 (Birds + Wood Elves), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_NormalDeckUnaffectedByCategoryRegex — regression
// test: a plain `1 Card` deck with no category headers still parses
// identically. The new regex must not steal real card lines.
func TestParseDeckReader_NormalDeckUnaffectedByCategoryRegex(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Spirited Companion
1 Lightning Bolt
1 Forest
1 Swamp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, typeCategoryMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 5 {
		t.Errorf("Library: want 5 (qty-prefixed cards must not match the bare-label regex), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	// Spirited Companion is a real card whose name overlaps the
	// "Companion" alt-board label — qty prefix saves it.
	found := false
	for _, c := range td.Library {
		if c.Name == "Spirited Companion" {
			found = true
		}
	}
	if !found {
		t.Errorf("Spirited Companion: lost to overlap with Companion label; library=%v", libraryNames(td.Library))
	}
}

// commanderNames extracts the names of every commander card on td.
// Helper to keep test assertions readable.
func commanderNames(td *TournamentDeck) []string {
	out := make([]string, 0, len(td.CommanderCards))
	for _, c := range td.CommanderCards {
		out = append(out, c.Name)
	}
	return out
}
