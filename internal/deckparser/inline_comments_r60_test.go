package deckparser

import (
	"strings"
	"testing"
)

// inline_comments_r60_test.go — inline `// comment` support audit.
//
// Deckbuilders annotate slots with intent notes: `1 Sol Ring // mvp`,
// `1 Cyclonic Rift // wincon — never cut`, `1 Bojuka Bog // gy hate
// flex slot`. Pre-fix, every such line landed in Unresolved because
// cleanCardName had no handler for the `//` token and `Sol Ring //
// mvp` didn't match the meta. Worse, the deckbuilder's intent note
// was lost entirely — no place to surface it in build-coaching UI.
//
// Fix splits each line on the first ` // ` token, captures the suffix
// as the line's Comment, and preserves the per-line view in a new
// td.CardLines slice (parallel to Library, which stays flattened
// across copies). Whole-line `//` comments (lines starting with `//`)
// are still dropped at the top of the parse loop — the inline split
// only catches `card // note` shapes.

func icMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{
		"Atraxa, Praetors' Voice", "Sol Ring", "Cyclonic Rift",
		"Lightning Bolt", "Bojuka Bog",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary", Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_InlineCommentNamesParse — the load-bearing fix:
// `card // comment` lines resolve to the correct card.
func TestParseDeckReader_InlineCommentNamesParse(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring // mvp
1 Cyclonic Rift // wincon — never cut
2 Lightning Bolt // burn package
1 Bojuka Bog
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 5 {
		t.Errorf("library: want 5 (1+1+2+1), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("inline comments should not produce unresolved entries, got %v", td.Unresolved)
	}
}

// TestParseDeckReader_InlineCommentRetained — Comment field captures
// the deckbuilder's note, indexed in CardLines.
func TestParseDeckReader_InlineCommentRetained(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring // mvp
1 Cyclonic Rift // wincon — never cut
2 Lightning Bolt // burn package
1 Bojuka Bog
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CardLines) != 4 {
		t.Fatalf("CardLines: want 4 (one per source line), got %d (%v)",
			len(td.CardLines), td.CardLines)
	}
	wants := []CardLine{
		{Qty: 1, Name: "Sol Ring", Comment: "mvp", Section: "main"},
		{Qty: 1, Name: "Cyclonic Rift", Comment: "wincon — never cut", Section: "main"},
		{Qty: 2, Name: "Lightning Bolt", Comment: "burn package", Section: "main"},
		{Qty: 1, Name: "Bojuka Bog", Comment: "", Section: "main"},
	}
	for i, want := range wants {
		got := td.CardLines[i]
		if got.Qty != want.Qty || got.Name != want.Name ||
			got.Comment != want.Comment || got.Section != want.Section {
			t.Errorf("CardLines[%d] = %+v, want %+v", i, got, want)
		}
	}
}

// TestParseDeckReader_WholeLineCommentStillDropped — a line starting
// with `//` is still a whole-line comment (pre-existing behavior).
// The inline-strip must not touch it; it short-circuits at the top of
// the parse loop. Defends against the new split swallowing real card
// content from a malformed `// 1 Sol Ring` line (which a user might
// type to temporarily disable a card).
func TestParseDeckReader_WholeLineCommentStillDropped(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
// commented-out: 1 Sol Ring // mvp
1 Cyclonic Rift
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 1 || td.Library[0].Name != "Cyclonic Rift" {
		t.Errorf("library: want [Cyclonic Rift] (Sol Ring disabled via whole-line //), got %v",
			libraryNames(td.Library))
	}
	// Sol Ring should NOT appear in CardLines either — the whole line
	// was dropped before the line loop reached the inline split.
	for _, cl := range td.CardLines {
		if cl.Name == "Sol Ring" {
			t.Errorf("whole-line comment leaked into CardLines: %+v", cl)
		}
	}
}

// TestParseDeckReader_InlineCommentEmptyAllowed — `1 Sol Ring //` with
// no comment text resolves to a card with empty Comment, not garbage.
func TestParseDeckReader_InlineCommentEmptyAllowed(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring //
1 Cyclonic Rift //
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2, got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	for _, cl := range td.CardLines {
		if cl.Comment != "" {
			t.Errorf("CardLines[%s].Comment = %q, want empty (no comment text after `//`)", cl.Name, cl.Comment)
		}
	}
}

// TestParseDeckReader_InlineCommentWithPrintingTail — interaction with
// the existing `(SET) 123` strip. Order matters: comment peels first
// (it's the rightmost element source-wise), then the printing tail.
func TestParseDeckReader_InlineCommentWithPrintingTail(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring (CMR) 250 // mvp
1 Cyclonic Rift [MM3] 36 // wincon
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 2 {
		t.Errorf("library: want 2 (printing tail + comment both peel), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	wantComments := map[string]string{"Sol Ring": "mvp", "Cyclonic Rift": "wincon"}
	for _, cl := range td.CardLines {
		if cl.Section != "main" {
			continue
		}
		if got := cl.Comment; got != wantComments[cl.Name] {
			t.Errorf("CardLines[%s].Comment = %q, want %q", cl.Name, got, wantComments[cl.Name])
		}
	}
}

// TestParseDeckReader_InlineCommentOnCommanderLine — Commander-section
// lines (Moxfield's `Commander\n1 Atraxa // primary general` shape)
// also carry comments; the Commander section path appends to
// commanderSectionNames AND CardLines with Section="commander".
func TestParseDeckReader_InlineCommentOnCommanderLine(t *testing.T) {
	text := `Commander
1 Atraxa, Praetors' Voice // primary general

Deck
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != "Atraxa, Praetors' Voice" {
		t.Fatalf("commander not parsed: %+v", td.CommanderCards)
	}
	var foundCmdComment bool
	for _, cl := range td.CardLines {
		if cl.Section == "commander" && cl.Name == "Atraxa, Praetors' Voice" {
			if cl.Comment != "primary general" {
				t.Errorf("commander CardLine.Comment = %q, want %q", cl.Comment, "primary general")
			}
			foundCmdComment = true
		}
	}
	if !foundCmdComment {
		t.Errorf("commander CardLine missing from CardLines: %+v", td.CardLines)
	}
}

// TestParseDeckReader_NegativeOfFix_PlainDeckUnchanged — a deck with
// no inline comments parses identically before and after the fix.
// CardLines is populated but Comment is empty everywhere; Library
// count and contents unchanged.
func TestParseDeckReader_NegativeOfFix_PlainDeckUnchanged(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring
1 Cyclonic Rift
2 Lightning Bolt
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, icMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 4 {
		t.Errorf("library: want 4 (1+1+2), got %d (%v)", len(td.Library), libraryNames(td.Library))
	}
	if len(td.CardLines) != 3 {
		t.Errorf("CardLines: want 3 (one per main-section line), got %d (%+v)",
			len(td.CardLines), td.CardLines)
	}
	for _, cl := range td.CardLines {
		if cl.Comment != "" {
			t.Errorf("plain deck should have empty Comment, got %q on %s", cl.Comment, cl.Name)
		}
	}
}
