package deckparser

import (
	"strings"
	"testing"
)

// dfc_inline_comment_r60_test.go — DFC face-separator vs inline-comment
// disambiguation audit.
//
// Moxfield's canonical export format names DFC / split / adventure cards
// with a ` // ` face separator: `1 Aang, Swift Savior // Aang and La,
// Ocean's Fury`. The pre-fix inlineCommentRE greedily matched this
// separator as the start of a user `// comment`, stripping the back-face
// name and leaving CardLine.Name = front-face only with CardLine.Comment
// carrying the back-face as a spurious "user note". Cards still resolved
// via buildCard's face-match fallback, so the bug was invisible to the
// engine — but build-coaching UIs that surface CardLine.Comment as the
// deckbuilder's intent note rendered nonsense ("Sundering Eruption — note:
// Volcanic Fissure") on ~3K lines across the curated corpus.
//
// Fix probes meta with the full pre-strip name; if it resolves as a known
// card, the ` // ` is the DFC separator and the strip is skipped.

func dfcMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	// Five real DFC / split / adventure cards picked from the moxfield/
	// corpus that previously failed (CardLine.Name lost the back-face).
	for _, n := range []string{
		"Aang, Swift Savior // Aang and La, Ocean's Fury",
		"Needleverge Pathway // Pillarverge Pathway",
		"Witch Enchanter // Witch-Blessed Meadow",
		"Brigid, Clachan's Heart // Brigid, Doun's Mind",
		"Kabira Takedown // Kabira Plateau",
		"Ondu Inversion // Ondu Skyruins",
		"Pinnacle Monk // Mystic Peak",
		"Sundering Eruption // Volcanic Fissure",
		"The Fall of Lord Konda // Fragment of Konda",
		// Plus a non-DFC card the inline-comment tests rely on.
		"Atraxa, Praetors' Voice",
		"Sol Ring",
		"Cyclonic Rift",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary", Types: []string{"legendary"}, CMC: 1,
		}
	}
	return meta
}

// TestParseDeckReader_DFCSeparatorPreservedInCardLine — the load-bearing
// fix. Five canonical DFC card lines from the real moxfield/ corpus that
// previously had their back-face stripped into Comment now preserve the
// full ` // ` name on CardLine.Name with empty Comment.
func TestParseDeckReader_DFCSeparatorPreservedInCardLine(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Aang, Swift Savior // Aang and La, Ocean's Fury
1 Needleverge Pathway // Pillarverge Pathway
1 Witch Enchanter // Witch-Blessed Meadow
1 Brigid, Clachan's Heart // Brigid, Doun's Mind
1 Kabira Takedown // Kabira Plateau
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, dfcMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantNames := []string{
		"Aang, Swift Savior // Aang and La, Ocean's Fury",
		"Needleverge Pathway // Pillarverge Pathway",
		"Witch Enchanter // Witch-Blessed Meadow",
		"Brigid, Clachan's Heart // Brigid, Doun's Mind",
		"Kabira Takedown // Kabira Plateau",
	}
	if len(td.CardLines) != len(wantNames) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(wantNames), len(td.CardLines), td.CardLines)
	}
	for i, want := range wantNames {
		got := td.CardLines[i]
		if got.Name != want {
			t.Errorf("CardLines[%d].Name = %q, want %q (back-face was stripped as comment pre-fix)", i, got.Name, want)
		}
		if got.Comment != "" {
			t.Errorf("CardLines[%d].Comment = %q, want empty (back-face leaked into Comment pre-fix)", i, got.Comment)
		}
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none, got %v", td.Unresolved)
	}
}

// TestParseDeckReader_DFCAndRealCommentCoexist — the disambiguation is
// surgical: real inline comments (`1 Sol Ring // mvp`) still strip
// correctly while DFC names (`1 Witch Enchanter // Witch-Blessed
// Meadow`) stay intact. Demonstrates the meta-probe distinction.
func TestParseDeckReader_DFCAndRealCommentCoexist(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Sol Ring // mvp
1 Witch Enchanter // Witch-Blessed Meadow
1 Cyclonic Rift // wincon — never cut
1 Sundering Eruption // Volcanic Fissure
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, dfcMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []CardLine{
		{Qty: 1, Name: "Sol Ring", Comment: "mvp", Section: "main"},
		{Qty: 1, Name: "Witch Enchanter // Witch-Blessed Meadow", Comment: "", Section: "main"},
		{Qty: 1, Name: "Cyclonic Rift", Comment: "wincon — never cut", Section: "main"},
		{Qty: 1, Name: "Sundering Eruption // Volcanic Fissure", Comment: "", Section: "main"},
	}
	if len(td.CardLines) != len(want) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(want), len(td.CardLines), td.CardLines)
	}
	for i, w := range want {
		got := td.CardLines[i]
		got.Status = 0
		got.LineNumber = 0
		if got != w {
			t.Errorf("CardLines[%d] = %+v, want %+v", i, got, w)
		}
	}
}

// TestParseDeckReader_DFCWithSetParens — Moxfield's full export shape
// pairs the DFC name with a `(SET) NUM` printing tail:
//
//	1 Aang, Swift Savior // Aang and La, Ocean's Fury (TLA) 12
//
// The DFC probe strips the set-parens tail before checking meta, so the
// disambiguation still works when the printing tail is present.
func TestParseDeckReader_DFCWithSetParens(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Aang, Swift Savior // Aang and La, Ocean's Fury (TLA) 12
1 Sundering Eruption // Volcanic Fissure (MID) 165
1 Ondu Inversion // Ondu Skyruins (ZNR) 230
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, dfcMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantNames := []string{
		"Aang, Swift Savior // Aang and La, Ocean's Fury",
		"Sundering Eruption // Volcanic Fissure",
		"Ondu Inversion // Ondu Skyruins",
	}
	if len(td.CardLines) != len(wantNames) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(wantNames), len(td.CardLines), td.CardLines)
	}
	for i, want := range wantNames {
		if td.CardLines[i].Name != want {
			t.Errorf("CardLines[%d].Name = %q, want %q", i, td.CardLines[i].Name, want)
		}
		if td.CardLines[i].Comment != "" {
			t.Errorf("CardLines[%d].Comment = %q, want empty (set-parens DFC leaked back-face into Comment)", i, td.CardLines[i].Comment)
		}
	}
}

// TestParseDeckReader_DFCFrontFaceOnlyStillCommentable — when the user
// writes only the front-face name plus a real comment
// (`1 Aang, Swift Savior // mvp`), the meta probe correctly returns nil
// for the full pre-strip string (no such card), so the suffix is
// preserved as a comment. The card resolves via buildCard's existing
// face-match fallback.
func TestParseDeckReader_DFCFrontFaceOnlyStillCommentable(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Aang, Swift Savior // mvp pick
1 Pinnacle Monk // flex slot
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, dfcMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []CardLine{
		{Qty: 1, Name: "Aang, Swift Savior", Comment: "mvp pick", Section: "main"},
		{Qty: 1, Name: "Pinnacle Monk", Comment: "flex slot", Section: "main"},
	}
	if len(td.CardLines) != len(want) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(want), len(td.CardLines), td.CardLines)
	}
	for i, w := range want {
		got := td.CardLines[i]
		got.Status = 0
		got.LineNumber = 0
		if got != w {
			t.Errorf("CardLines[%d] = %+v, want %+v", i, got, w)
		}
	}
	// The cards should still resolve — buildCard's face-match catches
	// the front-face-only form even though CardLine.Name is short.
	if len(td.Library) != 2 {
		t.Errorf("Library: want 2 (face-match should resolve front-face-only names), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
}

// TestParseDeckReader_DFCUnknownToMetaFallsBackToComment — when the
// ` // X` suffix probe misses meta (e.g. an obscure DFC card not in the
// loaded oracle), the strip falls back to the pre-fix path and treats
// the suffix as a comment. Documents that the disambiguation is
// best-effort: a card unknown to meta cannot be distinguished from a
// real user comment, and the existing buildCard face-match fallback
// still resolves the front face. Five distinct DFC names unknown to
// the small test meta exercise this path.
func TestParseDeckReader_DFCUnknownToMetaFallsBackToComment(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Some Obscure Card // Obscure Back Face
1 Another Mystery // Other Side
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, dfcMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// dfcMeta doesn't include these cards, so the probe returns nil and
	// the suffix strips as a comment (pre-fix behavior preserved).
	want := []CardLine{
		{Qty: 1, Name: "Some Obscure Card", Comment: "Obscure Back Face", Section: "main"},
		{Qty: 1, Name: "Another Mystery", Comment: "Other Side", Section: "main"},
	}
	if len(td.CardLines) != len(want) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(want), len(td.CardLines), td.CardLines)
	}
	for i, w := range want {
		got := td.CardLines[i]
		got.Status = 0
		got.LineNumber = 0
		if got != w {
			t.Errorf("CardLines[%d] = %+v, want %+v", i, got, w)
		}
	}
}
