package deckparser

import (
	"strings"
	"testing"
)

// deckbox_archidekt_mtggoldfish_r60_test.go — explicit support for the
// three major non-Moxfield decklist export formats.
//
// Pre-fix behavior per format:
//
//   - **Deckbox**: simple `qty\tname` lines parsed OK (existing `\s+`
//     in deckLineRE eats the tab), but the full-inventory CSV/TSV form
//     `qty\tname\tedition\tcondition\tlanguage\tfoil` left the extra
//     tab fields glued to the name (`Lightning Bolt\tM11`) and every
//     card landed in Unresolved.
//
//   - **Archidekt**: trailing `[Category]` annotation (`1 Sol Ring [Ramp]`,
//     `1 Atraxa [Commander{top}]`) was stripped by bracketTagRE as if
//     a set code — but the routing intent was lost. A deck where the
//     commander appeared mid-list (not first alphabetically) silently
//     auto-picked the wrong card as commander.
//
//   - **MTGGoldfish**: HTML pastes (`<br>`, `<div class="deck">`, `<td>`)
//     polluted every card line. `4 Lightning Bolt<br>` looked up
//     `Lightning Bolt<br>` in meta and failed.
//
// Fix layers three orthogonal additions: htmlTagRE strips well-formed
// `<...>` tags before any other tokenization (MTGGoldfish);
// deckboxTabExtraRE drops the second-and-beyond tab-separated fields
// after deckLineRE has matched (Deckbox); archidektCategoryRE +
// isArchidektCategoryLabel extract the trailing bracket category and
// route the line per-category (Commander → commander, Sideboard →
// sideboard count, Ramp/Removal/Creatures/etc. → mainboard with
// bracket peeled). The Archidekt extractor distinguishes from MTGO
// set-code brackets ([LEA] / [KHM]) by requiring at least one lowercase
// letter in the bracket content.

func multiFormatMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		// Commander-format staples.
		"Atraxa, Praetors' Voice",
		"Sol Ring", "Command Tower", "Arcane Signet", "Cyclonic Rift",
		"Lightning Greaves", "Counterspell", "Demonic Tutor",
		"Vampiric Tutor", "Birds of Paradise",
		// Modern / Legacy / Pauper staples for non-Commander tests.
		"Lightning Bolt", "Goblin Guide", "Snapcaster Mage", "Tarmogoyf",
		"Force of Will", "Brainstorm", "Ponder", "Preordain",
		"Path to Exile", "Searing Blaze", "Lava Spike", "Mulldrifter",
		// Basics.
		"Mountain", "Forest", "Island", "Plains", "Swamp",
		// A bracket-bait MTGO single-set name.
		"Bayou",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, Types: []string{"generic"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_DeckboxSimpleTabFormat — Deckbox's basic
// decklist export uses `qty\tname` (single tab). The existing
// deckLineRE `\s+` already handles tab as separator, but the test
// pins the behavior so any future regex tightening doesn't regress.
func TestParseDeckReader_DeckboxSimpleTabFormat(t *testing.T) {
	text := "COMMANDER: Atraxa, Praetors' Voice\n" +
		"4\tLightning Bolt\n" +
		"4\tGoblin Guide\n" +
		"24\tMountain\n"
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 32 {
		t.Errorf("Library: want 32 (4+4+24), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_DeckboxFullInventoryTSV — Deckbox's full
// inventory CSV/TSV export appends edition / condition / language /
// foil after the name (`4\tLightning Bolt\tM11\tNM\tEN\tfoil`). The
// new deckboxTabExtraRE strips fields 2-N so meta lookups succeed.
func TestParseDeckReader_DeckboxFullInventoryTSV(t *testing.T) {
	text := "COMMANDER: Atraxa, Praetors' Voice\n" +
		"4\tLightning Bolt\tM11\tNM\tEN\n" +
		"4\tGoblin Guide\tZEN\tNM\tEN\tfoil\n" +
		"1\tSnapcaster Mage\tISD\tNM\tEN\n" +
		"4\tForce of Will\tEMA\tLP\tEN\tfoil\n" +
		"24\tMountain\tM11\tNM\tEN\n"
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 37 {
		t.Errorf("Library: want 37 (4+4+1+4+24), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none (tab metadata should strip cleanly), got %v", td.Unresolved)
	}
}

// TestParseDeckReader_DeckboxSideboardHeader — Deckbox's section
// header convention uses colon (`Sideboard:`). The existing
// sectionHeaderRE matches with optional trailing colon, so the
// section routing already works — this test pins it for the format-
// detection plumbing (SideboardCount).
func TestParseDeckReader_DeckboxSideboardHeader(t *testing.T) {
	text := "4\tLightning Bolt\tM11\n" +
		"4\tGoblin Guide\tZEN\n" +
		"24\tMountain\tM11\n" +
		"4\tSearing Blaze\tWWK\n" +
		"4\tForest\n" +
		"4\tIsland\n" +
		"4\tPlains\n" +
		"4\tSwamp\n" +
		"4\tBrainstorm\n" +
		"4\tPonder\n" +
		"\nSideboard:\n" +
		"2\tSearing Blaze\tWWK\n" +
		"3\tPath to Exile\tISD\n" +
		"4\tPreordain\tM11\n" +
		"4\tCounterspell\tICE\n" +
		"2\tForce of Will\tEMA\n"
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if td.SideboardCount != 15 {
		t.Errorf("SideboardCount = %d, want 15", td.SideboardCount)
	}
	if td.DetectedFormat != FormatConstructed {
		t.Errorf("DetectedFormat = %q, want %q", td.DetectedFormat, FormatConstructed)
	}
}

// TestParseDeckReader_ArchidektCategoryBracketRouting — Archidekt's
// per-line `[Category]` annotation routes to the correct section:
// [Commander] → commander, [Sideboard] → sideboard count,
// [Ramp]/[Removal]/[Creatures]/[Lands] → mainboard (bracket peeled).
// The commander annotation overrides position-based commander
// detection — a deck where Zur appears mid-list (not first)
// correctly resolves Zur as commander instead of auto-picking the
// first alphabetical card.
func TestParseDeckReader_ArchidektCategoryBracketRouting(t *testing.T) {
	meta := multiFormatMeta()
	meta.byName[normalizeName("Zur the Enchanter")] = &CardMeta{
		Name: "Zur the Enchanter", Types: []string{"legendary", "creature"}, CMC: 4,
	}
	text := `1 Birds of Paradise (DDH) 49 [Ramp]
1 Sol Ring (CMR) 254 [Ramp]
1 Cyclonic Rift (CMR) 76 [Removal]
1 Counterspell (CMR) 67 [Sideboard]
1 Path to Exile [Sideboard]
1 Zur the Enchanter (CMR) 100 [Commander{top}]
1 Vampiric Tutor [Tutors]
1 Demonic Tutor [Tutors]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Zur the Enchanter" {
		t.Errorf("Commander: want [Zur the Enchanter] (Archidekt [Commander] bracket routes here), got %v",
			commanderNames(td))
	}
	if td.SideboardCount != 2 {
		t.Errorf("SideboardCount = %d, want 2 (Counterspell + Path to Exile)", td.SideboardCount)
	}
	// Mainboard should be 5: Birds, Sol Ring, Cyclonic Rift, Vampiric Tutor, Demonic Tutor.
	// (Sideboard cards are dropped from Library; commander is excluded.)
	if len(td.Library) != 5 {
		t.Errorf("Library: want 5 mainboard cards, got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_ArchidektModifierStripped — Archidekt sometimes
// emits a `{top}` / `{bottom}` / `{maybeboard}` modifier inside the
// bracket. The regex captures only the category name, ignoring the
// modifier. Verifies all modifier variants.
func TestParseDeckReader_ArchidektModifierStripped(t *testing.T) {
	text := `1 Atraxa, Praetors' Voice [Commander{top}]
1 Sol Ring [Ramp{maybeboard}]
1 Cyclonic Rift [Removal{bottom}]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander not picked: %v", commanderNames(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("Library: want 2 (Sol Ring + Cyclonic Rift), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_ArchidektMTGOSetCodeBracketStillStrips — the
// case-heuristic must NOT misidentify all-uppercase set codes as
// Archidekt categories. `[LEA]`, `[KHM]`, `[MM3]` are set codes that
// bracketTagRE strips as part of cleanCardName. Verifies both
// extraction paths coexist.
func TestParseDeckReader_ArchidektMTGOSetCodeBracketStillStrips(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Lightning Bolt [LEA]
1 Goblin Guide [ZEN]
1 Snapcaster Mage [MM3]
1 Bayou [LEA]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 4 {
		t.Errorf("Library: want 4 (set-code brackets stripped as MTGO), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_MTGGoldfishHTMLBR — MTGGoldfish "Save as HTML"
// pastes leak `<br>` tags. The new htmlTagRE strips them inline at
// the top of the parse loop so every card line resolves.
func TestParseDeckReader_MTGGoldfishHTMLBR(t *testing.T) {
	text := "COMMANDER: Atraxa, Praetors' Voice<br>" + "\n" +
		"4 Lightning Bolt<br>\n" +
		"4 Goblin Guide<br>\n" +
		"24 Mountain<br>\n" +
		"<br>\n" +
		"Sideboard<br>\n" +
		"2 Searing Blaze<br>\n"
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander: %v", commanderNames(td))
	}
	if len(td.Library) != 32 {
		t.Errorf("Library: want 32 (4+4+24), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if td.SideboardCount != 2 {
		t.Errorf("SideboardCount = %d, want 2", td.SideboardCount)
	}
}

// TestParseDeckReader_MTGGoldfishHTMLTableFormat — full HTML table
// dump (`<table>`, `<tr>`, `<td>`, `</tr>`, etc.). Every tag strips
// inline; lines that were ENTIRELY HTML collapse to empty and skip
// at the blank check. Card-bearing rows like
// `<td>4</td><td>Lightning Bolt</td>` should still resolve via the
// stripped-down form `4 Lightning Bolt`. Sized to a Modern-shape 60+
// mainboard so the format-autodetector's no-commander-allowed branch
// fires instead of the auto-pick fallback.
func TestParseDeckReader_MTGGoldfishHTMLTableFormat(t *testing.T) {
	text := `<table class="deck">
<tr><th>Mainboard</th></tr>
<tr><td>4</td><td>Lightning Bolt</td></tr>
<tr><td>4</td><td>Goblin Guide</td></tr>
<tr><td>4</td><td>Snapcaster Mage</td></tr>
<tr><td>4</td><td>Brainstorm</td></tr>
<tr><td>4</td><td>Ponder</td></tr>
<tr><td>4</td><td>Preordain</td></tr>
<tr><td>4</td><td>Force of Will</td></tr>
<tr><td>4</td><td>Path to Exile</td></tr>
<tr><td>4</td><td>Counterspell</td></tr>
<tr><td>4</td><td>Lava Spike</td></tr>
<tr><td>20</td><td>Mountain</td></tr>
</table>
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 60 {
		t.Errorf("Library: want 60 (Modern-shape mainboard), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if td.DetectedFormat != FormatConstructed {
		t.Errorf("DetectedFormat = %q, want %q", td.DetectedFormat, FormatConstructed)
	}
}

// TestParseDeckReader_MTGGoldfishDivWrap — MTGGoldfish's older
// `<div class="deck"> ... </div>` wrapper. Lines that were entirely
// HTML collapse; the inner card lines parse cleanly. 60-card Modern
// shape so the format-autodetector accepts the no-commander signal.
func TestParseDeckReader_MTGGoldfishDivWrap(t *testing.T) {
	text := `<div class="deck">
4 Lightning Bolt
4 Goblin Guide
4 Snapcaster Mage
4 Brainstorm
4 Ponder
4 Preordain
4 Force of Will
4 Path to Exile
4 Counterspell
24 Mountain

Sideboard
2 Searing Blaze
3 Path to Exile
4 Preordain
4 Counterspell
2 Force of Will
</div>
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 60 {
		t.Errorf("Library: want 60, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if td.SideboardCount != 15 {
		t.Errorf("SideboardCount = %d, want 15", td.SideboardCount)
	}
	if td.DetectedFormat != FormatConstructed {
		t.Errorf("DetectedFormat = %q, want %q", td.DetectedFormat, FormatConstructed)
	}
}

// TestParseDeckReader_MTGGoldfishPlainTextUnaffected — MTGGoldfish's
// "Download as Text" produces clean plain-text output. The HTML strip
// must be a no-op on this shape (the `ContainsRune('<')` guard skips
// the regex when no `<` is present).
func TestParseDeckReader_MTGGoldfishPlainTextUnaffected(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Command Tower
1 Lightning Greaves
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 3 {
		t.Errorf("Library: want 3, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_AllThreeFormatsCanCoexist — a single deck mixing
// shapes (a user copy-pasted from multiple sources into one file).
// Verifies the three new strips compose without stepping on each other.
func TestParseDeckReader_AllThreeFormatsCanCoexist(t *testing.T) {
	text := `1 Atraxa, Praetors' Voice (CMR) 222 [Commander{top}]
1 Sol Ring [Ramp]
4	Lightning Bolt	M11	NM	EN
4 Goblin Guide<br>
1 Counterspell [Sideboard]
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, multiFormatMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("Commander: %v", commanderNames(td))
	}
	// Mainboard: Sol Ring + 4 Lightning Bolt + 4 Goblin Guide = 9
	if len(td.Library) != 9 {
		t.Errorf("Library: want 9 (1+4+4), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if td.SideboardCount != 1 {
		t.Errorf("SideboardCount = %d, want 1 (Counterspell)", td.SideboardCount)
	}
}

// TestIsArchidektCategoryLabel_HeuristicTable — the case-heuristic that
// distinguishes Archidekt categories from MTGO set codes. All-uppercase
// content is a set code (fall through to bracketTagRE); any lowercase
// letter implies a word-category (route via archidektCategoryRE).
func TestIsArchidektCategoryLabel_HeuristicTable(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		why  string
	}{
		{"Commander", true, "builtin category"},
		{"Sideboard", true, "builtin category"},
		{"Ramp", true, "user-defined category"},
		{"Wincons", true, "user-defined category"},
		{"Creatures", true, "type sub-category"},
		{"LEA", false, "set code (all uppercase)"},
		{"KHM", false, "set code (all uppercase)"},
		{"MM3", false, "set code (alpha+digits, no lowercase)"},
		{"2X2", false, "set code (digit-led, no lowercase)"},
		{"", false, "empty"},
	}
	for _, tc := range cases {
		got := isArchidektCategoryLabel(tc.in)
		if got != tc.want {
			t.Errorf("isArchidektCategoryLabel(%q) = %v, want %v (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}
