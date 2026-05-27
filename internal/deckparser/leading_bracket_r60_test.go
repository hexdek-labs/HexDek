package deckparser

import (
	"strings"
	"testing"
)

// leading_bracket_r60_test.go — regressions for the r60 deckparser
// edge-case audit. Two gaps in cleanCardName's decoration strippers:
//
//   1. MTGO / Deckstats native export uses a LEADING bracketed set code:
//      `4 [LEA] Sol Ring`. The deckLineRE captures qty=4 / name="[LEA] Sol
//      Ring", and the pre-fix bracketTagRE only stripped TRAILING brackets,
//      so "[LEA] Sol Ring" survived through to the meta lookup and silently
//      landed in Unresolved. Every line of an MTGO-formatted decklist hits
//      this path → entire library drops to zero. Worst gap by impact.
//
//   2. Archidekt's etched-foil marker is `*F-Etched*` — the hyphen is part
//      of the marker. The pre-fix foilMarkerRE used `[A-Za-z]+` which
//      rejects `-`, so `Sol Ring *F-Etched*` failed strip and dropped the
//      card into Unresolved.
//
// Both fixes live in cleanCardName + the trailing-suffix regex block. The
// negative-of-the-fix tests confirm the pre-existing trailing-bracket
// strip is unchanged.

// -----------------------------------------------------------------------------
// MTGO / Deckstats leading [SET] format — the worst gap.
// -----------------------------------------------------------------------------

func TestParseDeckReader_MTGOLeadingBracketSetCode(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Mana Crypt", "Demonic Tutor"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
4 [LEA] Sol Ring
1 [MH3] Mana Crypt
1 [2X2] Demonic Tutor
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("ParseDeckReader: %v", err)
	}
	if len(td.Library) != 6 {
		t.Fatalf("MTGO leading-bracket format: want 6 cards (4 Sol Ring + 1 Mana Crypt + 1 Demonic Tutor), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("unresolved should be empty (every line is a known card), got %v", td.Unresolved)
	}
	// And no `[LEA] Sol Ring`-style phantom names should survive.
	for _, c := range td.Library {
		if strings.HasPrefix(c.Name, "[") {
			t.Errorf("library card retained leading bracket: %q", c.Name)
		}
	}
}

// TestCleanCardName_LeadingBracketStripped is the helper-level pin so a
// regex rewrite that breaks the strip stays caught even if the
// ParseDeckReader integration test is bypassed.
func TestCleanCardName_LeadingBracketStripped(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"[LEA] Sol Ring", "Sol Ring"},
		{"[MH3] Mana Crypt", "Mana Crypt"},
		{"[2X2] Demonic Tutor", "Demonic Tutor"},
		// Combined leading + trailing decoration peels both ends.
		{"[LEA] Sol Ring *F*", "Sol Ring"},
		{"[NEO] Eiganjo, Seat of the Empire [Land]", "Eiganjo, Seat of the Empire"},
		// No leading bracket — passthrough on the leading strip, trailing
		// strip still fires.
		{"Sol Ring [Ramp]", "Sol Ring"},
		// No brackets at all — full passthrough.
		{"Sol Ring", "Sol Ring"},
	}
	for _, tc := range cases {
		got := cleanCardName(tc.in)
		if got != tc.want {
			t.Errorf("cleanCardName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// Etched-foil marker — *F-Etched* sibling fix.
// -----------------------------------------------------------------------------

func TestParseDeckReader_EtchedFoilMarker(t *testing.T) {
	meta := &MetaDB{byName: map[string]*CardMeta{}}
	for _, n := range []string{"Tinybones, the Pickpocket", "Sol Ring", "Mana Crypt", "Cyclonic Rift"} {
		meta.byName[normalizeName(n)] = &CardMeta{
			Name: n, TypeLine: "Legendary Creature",
			Types: []string{"legendary", "creature"}, CMC: 1,
		}
	}
	text := `COMMANDER: Tinybones, the Pickpocket
1 Sol Ring *F-Etched*
1 Mana Crypt *Foil-Etched*
1 Cyclonic Rift *Etched*
`
	td, err := ParseDeckReader(strings.NewReader(text), nil, meta)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(td.Library) != 3 {
		t.Fatalf("library want 3 (etched-foil markers stripped), got %d (%v); unresolved=%v",
			len(td.Library), libraryNames(td.Library), td.Unresolved)
	}
	if len(td.Unresolved) != 0 {
		t.Errorf("unresolved should be empty, got %v", td.Unresolved)
	}
}

func TestCleanCardName_EtchedFoilStripped(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Sol Ring *F-Etched*", "Sol Ring"},
		{"Sol Ring *Foil-Etched*", "Sol Ring"},
		{"Sol Ring *Etched*", "Sol Ring"},
		// Non-etched markers still work — defends against over-narrowing.
		{"Sol Ring *F*", "Sol Ring"},
		{"Sol Ring *E*", "Sol Ring"},
		{"Sol Ring *Foil*", "Sol Ring"},
	}
	for _, tc := range cases {
		got := cleanCardName(tc.in)
		if got != tc.want {
			t.Errorf("cleanCardName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// -----------------------------------------------------------------------------
// Negative-of-the-fix: existing trailing-bracket + plain decoration paths
// continue to peel correctly after the leading-bracket addition.
// -----------------------------------------------------------------------------

func TestCleanCardName_NegativeOfFix_TrailingBracketStillStrips(t *testing.T) {
	// A trailing `[Burn]` category tag (Moxfield hand-edit) — the leading
	// strip must NOT eat the card name itself, only the bracket at start.
	got := cleanCardName("Lightning Bolt [Burn]")
	if got != "Lightning Bolt" {
		t.Errorf("trailing-bracket strip regressed: cleanCardName(%q) = %q", "Lightning Bolt [Burn]", got)
	}
}
