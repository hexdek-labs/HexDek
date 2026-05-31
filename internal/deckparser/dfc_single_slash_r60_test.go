package deckparser

import (
	"strings"
	"testing"
)

// dfc_single_slash_r60_test.go — single-slash DFC face separator
// normalization audit. Sibling to dfc_inline_comment_r60_test.go (PR
// #785 closed the ` // ` vs ` // comment` ambiguity; this closes the
// ` / ` legacy / Aetherhub / TappedOut export shape).
//
// Moxfield's canonical DFC export is double-slash:
//
//	1 Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight
//
// but several other tools (Aetherhub, legacy TappedOut, hand-typed
// list-view copies) render the same card with a single slash:
//
//	1 Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight
//
// Pre-fix this dropped the card into Unresolved because (a) the meta
// lookup misses (`Eirdu... / Isilu...` ≠ `Eirdu... // Isilu...`) and
// (b) buildCard's face-match scans for ` // ` entries and tests each
// face against the input — but the input contains ` / ` so neither
// face matches in isolation.
//
// Fix adds normalizeDFCSeparator: when a name contains ` / ` but not
// ` // `, probe meta with the substituted ` // ` form; on a hit,
// canonicalize. Returns the input unchanged when meta is nil, the
// substitution misses, or the name was already canonical.

func singleSlashDFCMeta() *MetaDB {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	// Real DFC / split / adventure / meld card names from the canonical
	// Scryfall corpus. Each is stored under its meta-canonical ` // `
	// form so the single-slash inputs in the tests must canonicalize
	// to find them.
	for _, n := range []string{
		// The corpus's lone single-slash case (Edge of Eternities Commander).
		"Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight",
		// Canonical Apocalypse split cards — the original splits that
		// gave the ` // ` notation its name.
		"Life // Death",
		"Fire // Ice",
		"Assault // Battery",
		// Adventure card (Throne of Eldraine onward — adventure creatures
		// are stored under `{Creature} // {Adventure}`).
		"Bonecrusher Giant // Stomp",
		// MDFC (Zendikar Rising).
		"Sea Gate Restoration // Sea Gate, Reborn",
		// Plus a non-DFC card for negative-case tests.
		"Atraxa, Praetors' Voice",
		"Sol Ring",
	} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary", Types: []string{"legendary"}, CMC: 2,
		}
	}
	return meta
}

// TestNormalizeDFCSeparator_FiveCanonicalCards — the load-bearing
// transformation. Five distinct single-slash inputs from across the
// Magic timeline (Edge of Eternities Commander, Apocalypse splits x3,
// Eldraine adventure) all canonicalize to the meta-stored ` // ` form.
func TestNormalizeDFCSeparator_FiveCanonicalCards(t *testing.T) {
	meta := singleSlashDFCMeta()
	cases := []struct {
		in, want string
	}{
		{
			"Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight",
			"Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight",
		},
		{"Life / Death", "Life // Death"},
		{"Fire / Ice", "Fire // Ice"},
		{"Assault / Battery", "Assault // Battery"},
		{"Bonecrusher Giant / Stomp", "Bonecrusher Giant // Stomp"},
		{
			"Sea Gate Restoration / Sea Gate, Reborn",
			"Sea Gate Restoration // Sea Gate, Reborn",
		},
	}
	for _, tc := range cases {
		got := normalizeDFCSeparator(tc.in, meta)
		if got != tc.want {
			t.Errorf("normalizeDFCSeparator(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNormalizeDFCSeparator_PassthroughCases — three negative cases
// where the function must NOT mutate the input: (a) already canonical
// ` // ` (PR #785's domain, not ours), (b) substitution misses meta
// (input was never a DFC), (c) meta is nil (no probe possible).
func TestNormalizeDFCSeparator_PassthroughCases(t *testing.T) {
	meta := singleSlashDFCMeta()
	cases := []struct {
		in, want string
		meta     *MetaDB
		why      string
	}{
		{"Life // Death", "Life // Death", meta, "already canonical ` // ` form, no work needed"},
		{"Some Made-Up Card / With Slash", "Some Made-Up Card / With Slash", meta, "substitution misses meta — leave the original alone rather than mangle"},
		{"Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight", "Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight", nil, "nil meta — no probe possible, caller's face-match is the only remaining fallback"},
		{"", "", meta, "empty input — defensive"},
		{"Sol Ring", "Sol Ring", meta, "no slash at all — pass through"},
	}
	for _, tc := range cases {
		got := normalizeDFCSeparator(tc.in, tc.meta)
		if got != tc.want {
			t.Errorf("normalizeDFCSeparator(%q): got %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

// TestParseDeckReader_SingleSlashDFCResolvesEndToEnd — the corpus-found
// `1 Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight (ECL) 286`
// case from data/decks/underwood/. Verifies the full pipeline:
// set-parens strip → cleanCardName → normalizeDFCSeparator → meta
// lookup. Pre-fix this card landed in Unresolved.
func TestParseDeckReader_SingleSlashDFCResolvesEndToEnd(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Eirdu, Carrier of Dawn / Isilu, Carrier of Twilight (ECL) 286
1 Life / Death
1 Fire / Ice
1 Bonecrusher Giant / Stomp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, singleSlashDFCMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	wantNames := []string{
		"Eirdu, Carrier of Dawn // Isilu, Carrier of Twilight",
		"Life // Death",
		"Fire // Ice",
		"Bonecrusher Giant // Stomp",
	}
	if len(td.CardLines) != len(wantNames) {
		t.Fatalf("CardLines: want %d, got %d (%+v)", len(wantNames), len(td.CardLines), td.CardLines)
	}
	for i, want := range wantNames {
		if td.CardLines[i].Name != want {
			t.Errorf("CardLines[%d].Name = %q, want %q (single-slash should canonicalize)", i, td.CardLines[i].Name, want)
		}
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none (every single-slash DFC should resolve), got %v", td.Unresolved)
	}
	if len(td.Library) != len(wantNames) {
		t.Errorf("Library: want %d, got %d (%v)", len(wantNames), len(td.Library), libraryNames(td.Library))
	}
}

// TestParseDeckReader_SingleSlashCommanderDirective — the underwood
// deck's commander line uses single-slash too:
//
//	COMMANDER: Ashling, Rekindled / Ashling, Rimebound
//
// The commander directive parse must also canonicalize so the lookup
// against meta succeeds. Without this, the deck's commander would land
// in Unresolved and ParseDeckReader returns "no commander found".
func TestParseDeckReader_SingleSlashCommanderDirective(t *testing.T) {
	meta := singleSlashDFCMeta()
	// Add the meld pair this test exercises.
	meta.byName[normalizeName("Ashling, Rekindled // Ashling, Rimebound")] = &CardMeta{
		Name: "Ashling, Rekindled // Ashling, Rimebound",
		Types: []string{"legendary", "creature"}, CMC: 5,
	}
	text := `COMMANDER: Ashling, Rekindled / Ashling, Rimebound
1 Sol Ring
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "Ashling, Rekindled // Ashling, Rimebound"
	if len(td.CommanderCards) == 0 || td.CommanderCards[0].Name != want {
		t.Errorf("CommanderCards[0].Name = %v, want %q (single-slash commander didn't canonicalize)",
			td.CommanderCards, want)
	}
}

// TestParseDeckReader_SingleSlashAndDoubleSlashCoexist — a deck mixing
// both shapes (some lines single-slash from a list-view paste, others
// double-slash from a card-detail-view paste) parses all of them. Also
// regression-pins that PR #785's ` // ` disambiguation logic still
// fires correctly when both shapes appear in the same deck.
func TestParseDeckReader_SingleSlashAndDoubleSlashCoexist(t *testing.T) {
	text := `COMMANDER: Atraxa, Praetors' Voice
1 Life / Death
1 Fire // Ice
1 Bonecrusher Giant / Stomp
1 Assault // Battery
1 Sol Ring // mvp
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, singleSlashDFCMeta())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []CardLine{
		{Qty: 1, Name: "Life // Death", Section: "main"},
		{Qty: 1, Name: "Fire // Ice", Section: "main"},
		{Qty: 1, Name: "Bonecrusher Giant // Stomp", Section: "main"},
		{Qty: 1, Name: "Assault // Battery", Section: "main"},
		{Qty: 1, Name: "Sol Ring", Comment: "mvp", Section: "main"},
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
	if len(td.Unresolved) != 0 {
		t.Errorf("Unresolved: want none, got %v", td.Unresolved)
	}
}

// TestNormalizeDFCSeparator_MissingMetaDoesNotMangle — a single-slash
// shape that ISN'T a real DFC (or whose meta hasn't been loaded) must
// pass through unchanged. Without this guard, a typo like
// "Sol Ring / Foo" would get force-substituted to "Sol Ring // Foo"
// and then dropped into Unresolved with the wrong name in the report,
// hiding the real error from the user.
func TestNormalizeDFCSeparator_MissingMetaDoesNotMangle(t *testing.T) {
	meta := singleSlashDFCMeta()
	got := normalizeDFCSeparator("Definitely Not A Real Card / Made Up Suffix", meta)
	want := "Definitely Not A Real Card / Made Up Suffix"
	if got != want {
		t.Errorf("normalizeDFCSeparator(non-DFC single-slash) = %q, want %q (must not force-substitute on a miss)",
			got, want)
	}
}
