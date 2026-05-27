package main

import "testing"

// r60 isRealCard regression tests. Audit context: choose_modal sat at
// 93.1% coverage in the pre-r60 by-mechanic report (61 "MISSING")
// entirely because cmd/parser-coverage's loadOracle filter diverged
// from scripts/parser.py:is_real_card. Investigating those 61 cards
// confirmed all were definitionally-skipped types (47 funny set-type
// joke/playtest cards, 14 scheme/plane/vanguard/conspiracy entries),
// zero real parser-handler gaps.
//
// These tests pin the alignment so a future change to the parser's
// is_real_card filter that doesn't update isRealCard here will be
// caught — the by-mechanic denominator skew was invisible until it
// surfaced as a fake "61 uncovered" headline.

// TestIsRealCard_AcceptsExpansionCard pins the happy path: a normal
// black-bordered expansion creature is included in the denominator.
func TestIsRealCard_AcceptsExpansionCard(t *testing.T) {
	e := oracleEntry{
		Name:        "Brightglass Gearhulk",
		TypeLine:    "Artifact Creature — Construct",
		SetType:     "expansion",
		BorderColor: "black",
	}
	if !isRealCard(e) {
		t.Errorf("isRealCard rejected a normal expansion card: %+v", e)
	}
}

// TestIsRealCard_RejectsFunnySetType pins the dominant pre-r60
// inflation source: 47 of the 61 "uncovered" choose_modal cards were
// funny set-type joke/playtest entries (Mystery Booster Playtest cmb2,
// Unknown Event unk, Sunday sunf, …). All must be filtered.
func TestIsRealCard_RejectsFunnySetType(t *testing.T) {
	e := oracleEntry{
		Name:        "Tarkir Charm",
		TypeLine:    "Instant",
		SetType:     "funny",
		BorderColor: "black",
	}
	if isRealCard(e) {
		t.Errorf("isRealCard accepted a funny set-type card: %+v", e)
	}
}

// TestIsRealCard_RejectsExcludedTypeLines pins the parser's type-line
// filter across all 6 substring excludes plus the word-boundary
// "plane" case. Each represents a non-Constructed-legal format.
func TestIsRealCard_RejectsExcludedTypeLines(t *testing.T) {
	cases := []struct {
		name     string
		typeLine string
	}{
		{"plane", "Plane — Tarkir"},                 // Planechase plane
		{"scheme", "Scheme"},                        // Archenemy scheme
		{"ongoing scheme", "Ongoing Scheme"},        // Archenemy ongoing
		{"phenomenon", "Phenomenon"},                // Planechase phenomenon
		{"vanguard", "Vanguard"},                    // 1996 Vanguard avatar
		{"conspiracy", "Conspiracy — Secret Mission"}, // Conspiracy draft
		{"dungeon", "Dungeon"},                      // Dungeon venture token
		{"token", "Token Creature — Soldier"},       // explicit token
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := oracleEntry{
				Name:        "Test " + tc.name,
				TypeLine:    tc.typeLine,
				SetType:     "expansion",
				BorderColor: "black",
			}
			if isRealCard(e) {
				t.Errorf("isRealCard accepted type_line %q (must be excluded)", tc.typeLine)
			}
		})
	}
}

// TestIsRealCard_PlaneWordBoundary pins the subtle word-boundary
// behavior: "plane" must reject Planechase planes WITHOUT also
// rejecting every planeswalker. A naive substring contains("plane")
// would silently nuke the entire planeswalker corpus.
func TestIsRealCard_PlaneWordBoundary(t *testing.T) {
	plane := oracleEntry{
		Name:        "Some Plane",
		TypeLine:    "Plane — Dominaria",
		SetType:     "expansion",
		BorderColor: "black",
	}
	if isRealCard(plane) {
		t.Errorf("isRealCard accepted a Planechase plane: %+v", plane)
	}

	walker := oracleEntry{
		Name:        "Jace, the Mind Sculptor",
		TypeLine:    "Legendary Planeswalker — Jace",
		SetType:     "expansion",
		BorderColor: "black",
	}
	if !isRealCard(walker) {
		t.Errorf("isRealCard rejected a planeswalker — the plane filter is too aggressive: %+v", walker)
	}
}

// TestIsRealCard_RejectsSilverBordered pins the silver-border filter
// for older un-set cards that escaped via legitimate set_type fields
// (Unhinged / Unglued / Unstable predate the "funny" set_type tag).
func TestIsRealCard_RejectsSilverBordered(t *testing.T) {
	e := oracleEntry{
		Name:        "Cheatyface",
		TypeLine:    "Creature — Illusion",
		SetType:     "expansion",
		BorderColor: "silver",
	}
	if isRealCard(e) {
		t.Errorf("isRealCard accepted a silver-bordered card: %+v", e)
	}
}

// TestIsRealCard_RejectsMemorabilia pins the oversized-promo filter.
// These print as "Card // Card" double-faced entries for the 6"×8" art
// cards bundled with Commander products — they have full oracle text
// but aren't actual playable cards. Inflating the coverage denominator
// with them produces false "MISSING" hits.
func TestIsRealCard_RejectsMemorabilia(t *testing.T) {
	e := oracleEntry{
		Name:        "Noxious Hydra Breath",
		TypeLine:    "Sorcery",
		SetType:     "memorabilia",
		BorderColor: "black",
	}
	if isRealCard(e) {
		t.Errorf("isRealCard accepted a memorabilia card: %+v", e)
	}
}

// TestIsRealCard_RejectsMinigameAndToken pins the remaining two
// excludedSetTypes entries so a future shrink of the filter still
// fails this regression.
func TestIsRealCard_RejectsMinigameAndToken(t *testing.T) {
	for _, st := range []string{"minigame", "token"} {
		e := oracleEntry{
			Name:        "Test " + st,
			TypeLine:    "Sorcery",
			SetType:     st,
			BorderColor: "black",
		}
		if isRealCard(e) {
			t.Errorf("isRealCard accepted set_type=%q (must be excluded): %+v", st, e)
		}
	}
}

// TestIsRealCard_RejectsBioSuffix pins the " Bio" suffix filter — the
// Scryfall corpus includes character biography "cards" for Universes
// Beyond products (e.g. "Aragorn Bio") that have no rules text but a
// distinct entry.
func TestIsRealCard_RejectsBioSuffix(t *testing.T) {
	e := oracleEntry{
		Name:        "Aragorn Bio",
		TypeLine:    "Card",
		SetType:     "expansion",
		BorderColor: "black",
	}
	if isRealCard(e) {
		t.Errorf("isRealCard accepted a Bio-suffix entry: %+v", e)
	}
}

// TestPlaneTokenMatch pins the dash-tolerant tokenizer. Printed type
// lines use em-dash (U+2014) as the subtype separator; tests must
// cover that plus the en-dash fallback and the ASCII hyphen.
func TestPlaneTokenMatch(t *testing.T) {
	cases := []struct {
		typeLine string
		want     bool
	}{
		{"Plane — Dominaria", true},   // em-dash (canonical Scryfall)
		{"Plane – Dominaria", true},   // en-dash (some older imports)
		{"Plane - Dominaria", true},   // ASCII hyphen (manual edits)
		{"plane", true},               // bare token, lowercased
		{"Legendary Planeswalker — Jace", false},
		{"Creature — Plane Walker", true}, // "Plane" as standalone token
		{"Creature — Planewalker", false}, // glued substring, not a token
	}
	for _, tc := range cases {
		if got := planeTokenMatch(tc.typeLine); got != tc.want {
			t.Errorf("planeTokenMatch(%q) = %v, want %v", tc.typeLine, got, tc.want)
		}
	}
}
