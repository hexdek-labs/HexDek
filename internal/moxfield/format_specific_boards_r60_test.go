package moxfield

import (
	"encoding/json"
	"strings"
	"testing"
)

// format_specific_boards_r60_test.go — pins capture of Moxfield's
// format-specific gameplay boards: Planechase `planes`, Archenemy
// `schemes`, Oathbreaker `signatureSpells`, and Un-set
// `attractions` / `stickers` / `contraptions`.
//
// Pre-fix the importer dropped all six on the floor — a real-corpus
// audit of 50 cached precon decks (~/.cache/hexdek/moxfield) lost 80
// cards across 8 decks (10 planes each on the March of the Machine /
// Doctor Who Planechase precons, 10 schemes each on the Duskmourn
// Archenemy precons). Each is now emitted as a `//`-prefixed comment
// line so the downstream Commander deckparser silently skips it (no
// pollution of the playable 99) while the .txt round-trips the
// format-defining component intact.

// ---------------------------------------------------------------------------
// Planes — real shape mirrors the Doctor Who / March of the Machine precons
// (10 planes per deck under boards.planes.cards)
// ---------------------------------------------------------------------------

const fixturePlanes = `{
	"name": "Cavalry Charge (Planechase MoM precon-like)",
	"format": "commanderPrecons",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Kasla, the Broken Halo"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"planes": {
			"count": 10,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "Paliano"}},
				"b": {"quantity": 1, "card": {"name": "Inys Haen"}},
				"c": {"quantity": 1, "card": {"name": "Littjara"}}
			}
		}
	}
}`

func TestFixture_PlanesEmittedWithMarker(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixturePlanes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Plane: 1 Paliano",
		"// Plane: 1 Inys Haen",
		"// Plane: 1 Littjara",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing plane line %q in:\n%s", want, out)
		}
	}
}

func TestFixture_PlanesDoNotLeakIntoMainboard(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixturePlanes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, forbidden := range []string{
		"\n1 Paliano\n",
		"\n1 Inys Haen\n",
		"\n1 Littjara\n",
		"COMMANDER: Paliano",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("plane card leaked as bare mainboard/commander line %q in:\n%s", forbidden, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Schemes — real shape mirrors the Duskmourn Archenemy precons (10
// schemes per deck under boards.schemes.cards)
// ---------------------------------------------------------------------------

const fixtureSchemes = `{
	"name": "Endless Punishment (Archenemy Duskmourn precon-like)",
	"format": "commanderPrecons",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Niv-Mizzet, Visionary"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"schemes": {
			"count": 10,
			"cards": {
				"a": {"quantity": 1, "card": {"name": "I Will Savor Your Agony"}},
				"b": {"quantity": 2, "card": {"name": "Fear My Authority"}},
				"c": {"quantity": 1, "card": {"name": "I Call for Slaughter"}}
			}
		}
	}
}`

func TestFixture_SchemesEmittedWithMarkerAndQuantities(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureSchemes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Scheme: 1 I Will Savor Your Agony",
		"// Scheme: 2 Fear My Authority",
		"// Scheme: 1 I Call for Slaughter",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing scheme line %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Oathbreaker SignatureSpell — the load-bearing fix for Oathbreaker
// imports. The signature spell IS the deck's secondary commander-like
// component; silently dropping it leaves the deck non-functional.
// ---------------------------------------------------------------------------

const fixtureSignatureSpell = `{
	"name": "Teferi Time Raveler Oathbreaker (with Treasure Cruise)",
	"format": "oathbreaker",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Teferi, Time Raveler"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Brainstorm"}}}},
		"signatureSpells": {
			"count": 1,
			"cards": {
				"s": {"quantity": 1, "card": {"name": "Treasure Cruise"}}
			}
		}
	}
}`

func TestFixture_SignatureSpellEmittedWithMarker(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureSignatureSpell), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// SignatureSpell: 1 Treasure Cruise") {
		t.Errorf("missing signatureSpell line in:\n%s", out)
	}
	// Negative: Treasure Cruise must not appear as a bare mainboard
	// line — that would treat it as a 100th card in the regular library.
	if strings.Contains(out, "\n1 Treasure Cruise\n") {
		t.Errorf("signature spell leaked as mainboard line in:\n%s", out)
	}
	// And it absolutely must not get promoted to COMMANDER: (Teferi is
	// the Oathbreaker; Treasure Cruise is the signature spell).
	if strings.Contains(out, "COMMANDER: Treasure Cruise") {
		t.Errorf("signature spell promoted to commander in:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Un-set: attractions / stickers / contraptions — Unfinity, Unstable,
// Acorn-supported sets. Schema identical to companions/maybeboard; the
// real-corpus sample didn't have any (we only cached Commander precons)
// but the API exposes the slots and a user importing an Un-set deck
// would lose them.
// ---------------------------------------------------------------------------

const fixtureUnsetBoards = `{
	"name": "Unfinity-style attractions + stickers",
	"format": "commander",
	"boards": {
		"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Yenna, Redtooth Regent"}}}},
		"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
		"attractions": {
			"count": 2,
			"cards": {
				"a1": {"quantity": 1, "card": {"name": "Balloon Stand"}},
				"a2": {"quantity": 1, "card": {"name": "Bumper Cars"}}
			}
		},
		"stickers": {
			"count": 2,
			"cards": {
				"s1": {"quantity": 1, "card": {"name": "Ancestral, Awesome, Acidic Drake"}},
				"s2": {"quantity": 1, "card": {"name": "Battle-Scarred Goblin"}}
			}
		},
		"contraptions": {
			"count": 1,
			"cards": {
				"c1": {"quantity": 1, "card": {"name": "Accessories to Murder"}}
			}
		}
	}
}`

func TestFixture_UnsetBoardsEmittedWithMarkers(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureUnsetBoards), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for _, want := range []string{
		"// Attraction: 1 Balloon Stand",
		"// Attraction: 1 Bumper Cars",
		"// Sticker: 1 Ancestral, Awesome, Acidic Drake",
		"// Sticker: 1 Battle-Scarred Goblin",
		"// Contraption: 1 Accessories to Murder",
	} {
		if !containsLine(out, want) {
			t.Errorf("missing un-set board line %q in:\n%s", want, out)
		}
	}
}

// ---------------------------------------------------------------------------
// Determinism + section order — format-specific boards land AFTER the
// playable sections (commanders/mainboard/sideboard/companions/maybeboard/
// considering) so the deck's playable 99 stays at the top of the file.
// ---------------------------------------------------------------------------

func TestFixture_PlanesLandAfterPlayableSections(t *testing.T) {
	const mixed = `{
		"name": "Mixed: commander + mainboard + planes",
		"format": "commanderPrecons",
		"boards": {
			"commanders": {"count": 1, "cards": {"c": {"quantity": 1, "card": {"name": "Kasla, the Broken Halo"}}}},
			"mainboard":  {"count": 1, "cards": {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}}},
			"planes": {"count": 1, "cards": {"p": {"quantity": 1, "card": {"name": "Paliano"}}}}
		}
	}`
	var r apiResponse
	if err := json.Unmarshal([]byte(mixed), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	idxMain := strings.Index(out, "1 Sol Ring")
	idxPlane := strings.Index(out, "// Plane: 1 Paliano")
	if idxMain < 0 || idxPlane < 0 {
		t.Fatalf("missing one of the expected lines:\n%s", out)
	}
	if !(idxMain < idxPlane) {
		t.Errorf("ordering: mainboard@%d must come before planes@%d", idxMain, idxPlane)
	}
}

// Re-export determinism: two formatDecklist calls on the same data must
// produce byte-identical output (sorted keys, stable section order). A
// regression here would silently flip planar deck ordering across
// fetches.
func TestFixture_FormatSpecificBoardsAreDeterministic(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixturePlanes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	first, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	for i := 0; i < 20; i++ {
		out, err := formatDecklist(&r)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if out != first {
			t.Fatalf("formatDecklist not deterministic across calls; iter %d differs from first", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Pre-fix counterfactual — confirm the test setup actually reproduces
// the bug by checking the v3-wrapper accessor returns the expected
// content. If this passes but the formatDecklist tests above fail, the
// accessor wiring is wrong rather than the JSON shape.
// ---------------------------------------------------------------------------

func TestFixture_PlanesAccessorReturnsBoardContent(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixturePlanes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	planes := r.planes()
	if len(planes) != 3 {
		t.Errorf("planes accessor: want 3 entries (fixture has Paliano + Inys Haen + Littjara), got %d", len(planes))
	}
}

// ---------------------------------------------------------------------------
// Legacy top-level fallback — same as the other apiResponse accessors,
// signatureSpells/planes/schemes/etc. must fall back to the top-level
// keys when the v3 wrapper is absent. Mirrors the legacy fixture used
// for commanders/mainboard in fixtures_r60_test.go.
// ---------------------------------------------------------------------------

const fixtureLegacyTopLevelPlanes = `{
	"name": "Legacy top-level planes",
	"format": "commanderPrecons",
	"commanders": {"c": {"quantity": 1, "card": {"name": "Kasla, the Broken Halo"}}},
	"mainboard":  {"m": {"quantity": 1, "card": {"name": "Sol Ring"}}},
	"planes":     {"p1": {"quantity": 1, "card": {"name": "Paliano"}}}
}`

func TestFixture_LegacyTopLevelPlanesStillWork(t *testing.T) {
	var r apiResponse
	if err := json.Unmarshal([]byte(fixtureLegacyTopLevelPlanes), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := formatDecklist(&r)
	if err != nil {
		t.Fatalf("formatDecklist: %v", err)
	}
	if !containsLine(out, "// Plane: 1 Paliano") {
		t.Errorf("legacy top-level planes missing:\n%s", out)
	}
}
