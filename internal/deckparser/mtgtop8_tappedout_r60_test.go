package deckparser

import (
	"strings"
	"testing"
)

// mtgtop8_tappedout_r60_test.go — explicit format support for
// MTGTop8 and Tappedout exports.
//
// MTGTop8 emits decks in several formats (Apprentice / MWS / Plain /
// Cockatrice); the plain-text / MWS shapes are already largely handled
// (leadBracketTagRE handles `[M11]` set codes, sbPrefixRE handles
// `SB:` sideboard lines, cmdrHeaderCommentRE handles `// COMMANDER`).
// This PR adds: `// NAME:` / `// FORMAT:` / `// CREATOR:` preamble
// lines feed into SourceHints (previously dropped silently as `//`
// comments).
//
// Tappedout has more distinct gaps: the `#!<section>` hash-bang
// directive shape (`#!Commander`, `#!Mainboard`, `#!Sideboard`), the
// markdown `**Bold Header**` section wrapping, bullet-list line
// prefixes (`* 1 Card`, `- 1 Card`, `• 1 Card`), and trailing per-card
// price tags (`$1.50` / `€1,50` / `£2.00 GBP`).

func mtgtop8TappedoutMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sigarda, Host of Herons", "Chulane, Teller of Tales",
		"Sol Ring", "Command Tower", "Arcane Signet", "Lightning Greaves",
		"Cyclonic Rift", "Counterspell", "Force of Will", "Brainstorm",
		"Ponder", "Preordain", "Path to Exile", "Searing Blaze",
		"Lightning Bolt", "Goblin Guide", "Snapcaster Mage", "Tarmogoyf",
		"Lava Spike", "Mulldrifter", "Daze",
		"Sensei's Divining Top", "Birds of Paradise", "Eternal Witness",
		"Sylvan Library", "Demonic Tutor", "Vampiric Tutor",
		"Forest", "Mountain", "Island", "Plains", "Swamp",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_MTGTop8_MWSFormat — Magic Workstation export
// shape: `// NAME:` / `// CREATOR:` / `// FORMAT:` metadata
// preamble, leading `[SET]` set codes on every card, `SB:` prefix on
// sideboard lines. The metadata lines now feed SourceHints instead
// of being dropped silently.
func TestParseDeckReader_MTGTop8_MWSFormat(t *testing.T) {
	text := `// Deck file for Magic Workstation (http://www.magicworkstation.com)
// NAME: Burn
// CREATOR: kn0wL3DG3
// FORMAT: Modern
// Mainboard
4 [M11] Lightning Bolt
4 [ZEN] Goblin Guide
4 [WWK] Searing Blaze
4 [CHK] Lava Spike
4 [ISD] Snapcaster Mage
4 [M11] Brainstorm
4 [M11] Ponder
4 [M11] Preordain
4 [ISD] Path to Exile
4 [M11] Counterspell
20 [M11] Mountain
// Sideboard
SB: 3 [M11] Searing Blaze
SB: 2 [ISD] Path to Exile
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 60 {
		t.Errorf("Library: want 60 (Modern-shape mainboard, no auto-pick commander), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if td.SideboardCount != 5 {
		t.Errorf("SideboardCount = %d, want 5", td.SideboardCount)
	}
	// SourceHints should carry the preamble metadata.
	wantHints := []string{"NAME:", "CREATOR:", "FORMAT:"}
	hintsJoined := strings.Join(td.SourceHints, " | ")
	for _, want := range wantHints {
		if !strings.Contains(hintsJoined, want) {
			t.Errorf("SourceHints missing %q; got %v", want, td.SourceHints)
		}
	}
}

// TestParseDeckReader_MTGTop8_PlainText — MTGTop8's "Plain text"
// export option. No bracket set codes, no `//` metadata, just
// `qty Card` lines + `Sideboard` header. Already supported by
// existing handlers — pinned here as a coexistence check.
func TestParseDeckReader_MTGTop8_PlainText(t *testing.T) {
	text := `4 Lightning Bolt
4 Goblin Guide
4 Snapcaster Mage
4 Brainstorm
4 Ponder
4 Preordain
4 Force of Will
4 Path to Exile
4 Counterspell
4 Daze
20 Island

Sideboard
4 Mulldrifter
4 Tarmogoyf
3 Searing Blaze
2 Lava Spike
2 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 60 {
		t.Errorf("Library: want 60, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 15 {
		t.Errorf("SideboardCount = %d, want 15", td.SideboardCount)
	}
}

// TestParseDeckReader_MTGTop8_ApprenticeFormat — Apprentice's plain
// shape with `// FORMAT:` metadata. Covers the third MTGTop8 export
// variant. Apprentice doesn't bracket set codes (unlike MWS).
func TestParseDeckReader_MTGTop8_ApprenticeFormat(t *testing.T) {
	text := `// Apprentice 1.x decklist
// NAME: Storm
// CREATOR: anonymous
// FORMAT: Legacy
4 Sensei's Divining Top
4 Brainstorm
4 Ponder
4 Preordain
4 Force of Will
4 Lightning Bolt
4 Counterspell
4 Snapcaster Mage
4 Tarmogoyf
24 Island

SB: 4 Force of Will
SB: 3 Daze
SB: 2 Path to Exile
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 60 {
		t.Errorf("Library: want 60, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 9 {
		t.Errorf("SideboardCount = %d, want 9", td.SideboardCount)
	}
}

// TestParseDeckReader_Tappedout_HashBangDirectives — Tappedout's
// `#!Commander` / `#!Mainboard` / `#!Sideboard` directive shape.
// Pre-fix the parser treated these as generic `#` comments and
// dropped them silently, with the result that every subsequent
// card landed in the mainboard regardless of intent (so an
// `#!Sideboard` section had its cards leak into Library).
func TestParseDeckReader_Tappedout_HashBangDirectives(t *testing.T) {
	text := `#!Commander
1 Sigarda, Host of Herons

#!Mainboard
1 Sol Ring
1 Command Tower
1 Lightning Greaves
1 Cyclonic Rift

#!Sideboard
1 Counterspell
1 Path to Exile
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Errorf("Commander: %v", commanderNames(td))
	}
	if len(td.Library) != 4 {
		t.Errorf("Library: want 4 (Sol Ring + Command Tower + Lightning Greaves + Cyclonic Rift), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 2 {
		t.Errorf("SideboardCount = %d, want 2", td.SideboardCount)
	}
}

// TestParseDeckReader_Tappedout_BulletList — Tappedout's bullet-list
// export shape: every card prefixed with `* `, `- `, or `• `. The
// new bulletPrefixRE strips the leading bullet so the qty + name
// extraction works normally.
func TestParseDeckReader_Tappedout_BulletList(t *testing.T) {
	text := `COMMANDER: Chulane, Teller of Tales
* 1 Sol Ring
* 1 Command Tower
* 1 Lightning Greaves
- 1 Cyclonic Rift
- 1 Counterspell
• 1 Demonic Tutor
• 1 Birds of Paradise
* 4 Forest
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 11 {
		t.Errorf("Library: want 11 (1+1+1+1+1+1+1+4), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_Tappedout_MarkdownHeaders — Tappedout sometimes
// wraps section labels in markdown bold: `**Commanders (1):**`,
// `**Lands (38):**`. The new markdownBoldHeaderRE strips the `**`
// wrapper so sectionHeaderRE / typeCategoryHeaderRE can match the
// bare label underneath.
func TestParseDeckReader_Tappedout_MarkdownHeaders(t *testing.T) {
	text := `**Commanders (1):**
1 Atraxa, Praetors' Voice

**Creatures (3):**
1 Birds of Paradise
1 Eternal Witness
1 Snapcaster Mage

**Lands (3):**
1 Command Tower
1 Forest
1 Island

**Sideboard (2):**
1 Counterspell
1 Path to Exile
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander: %v", commanderNames(td))
	}
	if len(td.Library) != 6 {
		t.Errorf("Library: want 6 (3 creatures + 3 lands), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 2 {
		t.Errorf("SideboardCount = %d, want 2", td.SideboardCount)
	}
}

// TestParseDeckReader_Tappedout_PriceTags — Tappedout's
// per-card-price annotation: `1 Sol Ring $1.50`, `1 Counterspell
// €0,50`, `1 Force of Will $80.00 USD`. New priceTagRE strips the
// trailing price so the name lookup succeeds.
func TestParseDeckReader_Tappedout_PriceTags(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring $1.50
1 Command Tower $0.99
1 Force of Will $80.00 USD
1 Lightning Greaves €2,50
1 Cyclonic Rift £15.00 GBP
1 Demonic Tutor $25.00
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 6 {
		t.Errorf("Library: want 6 (every priced line resolves), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none (price tags should strip), got %v", td.Unresolved)
	}
}

// TestParseDeckReader_Tappedout_FullCombinedExport — a Tappedout
// export combining ALL its quirks: `#!` directives + markdown
// headers + bullets + price tags + per-line *CMDR* markers — in
// one file. Verifies the new handlers compose without stepping on
// each other.
func TestParseDeckReader_Tappedout_FullCombinedExport(t *testing.T) {
	text := `**Commanders (1):**
#!Commander
* 1 Sigarda, Host of Herons $5.00

**Mainboard (5):**
#!Mainboard
* 1 Sol Ring $1.50
* 1 Command Tower $0.99
- 1 Lightning Greaves €2,50
- 1 Cyclonic Rift £15.00 GBP
• 1 Demonic Tutor $25.00

**Sideboard (1):**
#!Sideboard
* 1 Counterspell $0.50
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Errorf("Commander: %v", commanderNames(td))
	}
	if len(td.Library) != 5 {
		t.Errorf("Library: want 5 (mainboard cards), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 1 {
		t.Errorf("SideboardCount = %d, want 1", td.SideboardCount)
	}
}

// TestParseDeckReader_Tappedout_HashBangDoesNotShadowComments — a
// regular `#` comment (not `#!`) still drops silently and feeds
// SourceHints when leading. Pins that the `#!` directive handling
// doesn't accidentally swallow normal `# Source:` lines.
func TestParseDeckReader_Tappedout_HashBangDoesNotShadowComments(t *testing.T) {
	text := `# Source: https://tappedout.net/mtg-decks/atraxa-superfriends/
# Imported on 2026-05-31
COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.SourceHints) != 2 {
		t.Errorf("SourceHints: want 2 (leading `# Source:` + `# Imported:`), got %d (%v)",
			len(td.SourceHints), td.SourceHints)
	}
	if len(td.CommanderCards) != 1 {
		t.Errorf("Commander: %v", commanderNames(td))
	}
}

// TestParseDeckReader_NegativeOfFix_BulletDoesNotEatCmdrMarker —
// `*CMDR*` is a single-asterisk-wrapped inline marker; `* ` is the
// bullet prefix. The bullet strip requires whitespace AFTER the
// bullet character, so `*CMDR*` stays intact for cmdrInlineMarkerRE
// to handle.
func TestParseDeckReader_NegativeOfFix_BulletDoesNotEatCmdrMarker(t *testing.T) {
	text := `1 Sigarda, Host of Herons *CMDR*
1 Sol Ring
1 Command Tower
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Errorf("Commander: want Sigarda (via *CMDR* marker, not eaten by bullet strip); got %v", commanderNames(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("Library: want 2 (Sol Ring + Command Tower), got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_NegativeOfFix_NoPriceStripWithoutCurrency —
// a card name ending in `2.00` (not a price; defensive — no real
// card has this but the strip must require a currency symbol).
// Strips ONLY when the trailing token starts with `$` / `€` / `£`.
func TestParseDeckReader_NegativeOfFix_NoPriceStripWithoutCurrency(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring 2.00
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, mtgtop8TappedoutMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// "Sol Ring 2.00" without currency symbol should NOT have the
	// "2.00" stripped — meta.Get("Sol Ring 2.00") will miss and the
	// card lands in Unresolved. (We don't have "Sol Ring 2.00" in
	// meta so it's expected to be unresolved.)
	if len(td.Unresolved) != 1 || td.Unresolved[0] != "Sol Ring 2.00" {
		t.Errorf("Unresolved: want [Sol Ring 2.00] (no-currency strip should NOT fire); got %v", td.Unresolved)
	}
}
