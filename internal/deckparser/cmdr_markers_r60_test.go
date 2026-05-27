package deckparser

import (
	"strings"
	"testing"
)

// cmdr_markers_r60_test.go — commander-line marker support audit.
//
// Two new commander markers:
//
//   1. Inline `*CMDR*` / `*Commander*` on the card line itself:
//      `1 Sigarda, Host of Herons *CMDR*`. Sibling to the existing
//      foilMarkerRE strip but with semantic meaning — when matched
//      the card is promoted to commander instead of mainboard.
//
//   2. `// COMMANDER` / `// CMDR` directive-comment header on the
//      preceding line: `// COMMANDER\n1 Sigarda, Host of Herons`.
//      Pre-fix, every whole-line `//` comment was silently dropped at
//      the top of the parse loop, so this directive was unreachable.
//      Fix detects the directive BEFORE the generic drop and sets a
//      pending flag that the next card line consumes.
//
// Both routes augment the existing COMMANDER: / PARTNER: directives
// and `Commander` section-header path — they do not replace them. A
// deck with no marker but a COMMANDER: footer still works.

func cmdrMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Sigarda, Host of Herons", "Atraxa, Praetors' Voice", "Thrasios, Triton Hero",
		"Tymna the Weaver", "Sol Ring", "Cyclonic Rift",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_InlineCMDRMarker — `*CMDR*` after the card name
// flags the card as commander; the marker is stripped from the name.
func TestParseDeckReader_InlineCMDRMarker(t *testing.T) {
	text := `1 Sigarda, Host of Herons *CMDR*
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Fatalf("commander: want Sigarda, got %v", commanderNamesProbe(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2 (Sol Ring + Cyclonic Rift), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
	// Sigarda should NOT be in library — pulled out as commander.
	for _, c := range td.Library {
		if c.Name == "Sigarda, Host of Herons" {
			t.Errorf("Sigarda leaked into library — *CMDR* marker didn't promote")
		}
	}
}

// TestParseDeckReader_InlineCMDRMarkerVariants — case-insensitive +
// `*Commander*` long form.
func TestParseDeckReader_InlineCMDRMarkerVariants(t *testing.T) {
	cases := []string{
		"1 Sigarda, Host of Herons *CMDR*",
		"1 Sigarda, Host of Herons *cmdr*",
		"1 Sigarda, Host of Herons *Commander*",
		"1 Sigarda, Host of Herons *COMMANDER*",
	}
	for _, line := range cases {
		text := line + "\n1 Sol Ring\n"
		td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
		if err != nil {
			t.Errorf("parse %q: %v", line, err)
			continue
		}
		if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
			t.Errorf("%q: commander not detected, got %v", line, commanderNamesProbe(td))
		}
	}
}

// TestParseDeckReader_CMDRHeaderComment — `// COMMANDER` on the line
// before a card flags it as commander.
func TestParseDeckReader_CMDRHeaderComment(t *testing.T) {
	text := `// COMMANDER
1 Sigarda, Host of Herons
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Fatalf("commander: want Sigarda, got %v", commanderNamesProbe(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_CMDRHeaderConsumesOnce — the pending flag is
// consumed by the first card line that follows. A blank line or
// regular comment between the header and the card should NOT consume
// it; subsequent card lines should NOT inherit the flag.
func TestParseDeckReader_CMDRHeaderConsumesOnce(t *testing.T) {
	text := `// COMMANDER

// some other comment
1 Sigarda, Host of Herons
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Fatalf("commander: want Sigarda after blanks/comments, got %v", commanderNamesProbe(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2 (only first card after header is commander), got %d (%v)",
			len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_CMDRHeaderPartnerPair — two `// COMMANDER`
// headers + two cards = partner pair.
func TestParseDeckReader_CMDRHeaderPartnerPair(t *testing.T) {
	text := `// COMMANDER
1 Thrasios, Triton Hero
// COMMANDER
1 Tymna the Weaver
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 2 {
		t.Fatalf("expected partner pair (2 commanders), got %d: %v",
			len(td.CommanderCards), commanderNamesProbe(td))
	}
	gotNames := map[string]bool{}
	for _, c := range td.CommanderCards {
		gotNames[c.Name] = true
	}
	if !gotNames["Thrasios, Triton Hero"] || !gotNames["Tymna the Weaver"] {
		t.Errorf("partner pair names wrong: got %v", commanderNamesProbe(td))
	}
}

// TestParseDeckReader_InlineCMDRWithComment — inline marker + inline
// `// note` comment on the same line both peel.
func TestParseDeckReader_InlineCMDRWithComment(t *testing.T) {
	text := `1 Sigarda, Host of Herons *CMDR* // primary general
1 Sol Ring // mvp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Fatalf("commander: %v", commanderNamesProbe(td))
	}
	var foundCmdComment bool
	for _, cl := range td.CardLines {
		if cl.Section == "commander" && cl.Name == "Sigarda, Host of Herons" {
			if cl.Comment != "primary general" {
				t.Errorf("commander comment: got %q, want %q", cl.Comment, "primary general")
			}
			foundCmdComment = true
		}
	}
	if !foundCmdComment {
		t.Errorf("commander CardLine missing: %+v", td.CardLines)
	}
}

// TestParseDeckReader_NegativeOfFix_NormalDeckUnchanged — a deck using
// the existing COMMANDER: directive (no marker, no header comment)
// parses identically.
func TestParseDeckReader_NegativeOfFix_NormalDeckUnchanged(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) != 1 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Errorf("legacy COMMANDER: directive regressed: %v", commanderNamesProbe(td))
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_NegativeOfFix_WholeLineCommentStillDrops — a
// generic `// some note` comment (not the COMMANDER directive form)
// must still be dropped, NOT trigger pendingCommanderHeader.
func TestParseDeckReader_NegativeOfFix_WholeLineCommentStillDrops(t *testing.T) {
	text := `// just a note about the deck
1 Sigarda, Host of Herons
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, cmdrMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// With no marker / no `// COMMANDER` header, the first card is the
	// implicit commander per existing fallback behavior.
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Sigarda, Host of Herons" {
		t.Errorf("expected implicit-first-card commander fallback, got %v", commanderNamesProbe(td))
	}
	// Sol Ring should be in library — `// just a note` should NOT
	// have flagged it as commander.
	if len(td.Library) != 1 || td.Library[0].Name != "Sol Ring" {
		t.Errorf("library: want [Sol Ring], got %v", libraryNames(td.Library))
	}
}

func commanderNamesProbe(td *TournamentDeck) []string {
	var out []string
	for _, c := range td.CommanderCards {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}
