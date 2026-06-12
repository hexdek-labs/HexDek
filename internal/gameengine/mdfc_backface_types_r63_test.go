package gameengine

// Regression tests for the metadata-less MDFC back-face type leak
// (mdfc-backface r63 — correctness-score baseline's single
// STATE-INTEGRITY offender, 28 of 300 games).
//
// The chaos harnesses (cmd/hexdek-loki, cmd/hexdek-correctness) build
// Cards without populating BackFace* fields, so IsMDFC() is false and
// EnsureMDFCBackFaceForBattlefield no-ops; StripAdventureHalfTypes then
// refuses because the front half ("sorcery") has no permanent type.
// Result: spell//land MDFCs played as lands entered the battlefield
// with Types ["sorcery", "//", "land", ...] and tripped Feynman's
// permanent_types invariant (CR §304.4/§307.1 — instants and sorceries
// cannot be permanents).
//
// StripToBackHalfTypes is the new EnsureBattlefieldFrontFace arm: when
// only the BACK half carries a permanent type, the back half is the
// only legal battlefield identity — keep it.

import (
	"reflect"
	"testing"
)

func TestStripToBackHalfTypes_SpellLandMDFC(t *testing.T) {
	// Agadeem's Awakening // Agadeem, the Undercrypt as the chaos
	// harness builds it: combined types, NO BackFace metadata.
	c := &Card{
		Name:     "Agadeem's Awakening // Agadeem, the Undercrypt",
		Types:    []string{"sorcery", "//", "land"},
		TypeLine: "sorcery // land",
	}
	if !StripToBackHalfTypes(c) {
		t.Fatal("StripToBackHalfTypes returned false for spell//land shape")
	}
	if !reflect.DeepEqual(c.Types, []string{"land"}) {
		t.Errorf("Types = %v, want [land]", c.Types)
	}
	if c.TypeLine != "land" {
		t.Errorf("TypeLine = %q, want %q", c.TypeLine, "land")
	}
}

func TestStripToBackHalfTypes_InstantCreatureDFC(t *testing.T) {
	// Visitor from Planet Q shape: instant front, creature back.
	c := &Card{
		Name:     "Visitor from Planet Q",
		Types:    []string{"instant", "//", "creature", "alien"},
		TypeLine: "instant // creature — alien",
	}
	if !StripToBackHalfTypes(c) {
		t.Fatal("StripToBackHalfTypes returned false for instant//creature shape")
	}
	if !reflect.DeepEqual(c.Types, []string{"creature", "alien"}) {
		t.Errorf("Types = %v, want [creature alien]", c.Types)
	}
	if c.TypeLine != "creature — alien" {
		t.Errorf("TypeLine = %q, want %q", c.TypeLine, "creature — alien")
	}
}

func TestStripToBackHalfTypes_RefusesFrontPermanentType(t *testing.T) {
	// Adventure shape — front half has the permanent type; this is
	// StripAdventureHalfTypes territory and the back-half arm must not
	// touch it.
	c := &Card{
		Name:     "Adventurous Eater // Have a Bite",
		Types:    []string{"creature", "human", "//", "sorcery"},
		TypeLine: "creature — human // sorcery",
	}
	if StripToBackHalfTypes(c) {
		t.Fatal("StripToBackHalfTypes mutated a front-permanent-type card")
	}
}

func TestStripToBackHalfTypes_RefusesSpellSpellSplit(t *testing.T) {
	// Fire // Ice — neither half is a permanent type. Refuse so the
	// §205 violation surfaces instead of being silently scrubbed.
	c := &Card{
		Name:     "Fire // Ice",
		Types:    []string{"instant", "//", "instant"},
		TypeLine: "instant // instant",
	}
	if StripToBackHalfTypes(c) {
		t.Fatal("StripToBackHalfTypes mutated a spell//spell split card")
	}
	if !reflect.DeepEqual(c.Types, []string{"instant", "//", "instant"}) {
		t.Errorf("Types mutated: %v", c.Types)
	}
}

func TestStripToBackHalfTypes_RefusesTokensAndNoLeak(t *testing.T) {
	tok := &Card{
		Name:     "Treasure",
		Types:    []string{"token", "artifact"},
		TypeLine: "token artifact",
	}
	if StripToBackHalfTypes(tok) {
		t.Error("mutated a token card")
	}
	plain := &Card{Name: "Swamp", Types: []string{"land", "swamp"}}
	if StripToBackHalfTypes(plain) {
		t.Error("mutated a card with no // leak")
	}
	if StripToBackHalfTypes(nil) {
		t.Error("mutated nil")
	}
}

func TestEnsureBattlefieldFrontFace_MetadataLessMDFC(t *testing.T) {
	// The full composed hook on the exact chaos-harness shape: no
	// BackFace metadata, so the swap and the adventure stripper both
	// no-op and the back-half arm must clean up.
	c := &Card{
		Name:     "Malakir Rebirth // Malakir Mire",
		Types:    []string{"instant", "//", "land"},
		TypeLine: "instant // land",
	}
	EnsureBattlefieldFrontFace(c)
	if !reflect.DeepEqual(c.Types, []string{"land"}) {
		t.Errorf("Types = %v, want [land]", c.Types)
	}
	for _, typ := range c.Types {
		if typ == "instant" || typ == "sorcery" {
			t.Errorf("spell type %q leaked onto battlefield identity", typ)
		}
	}
	// Idempotent.
	EnsureBattlefieldFrontFace(c)
	if !reflect.DeepEqual(c.Types, []string{"land"}) {
		t.Errorf("second call changed Types = %v", c.Types)
	}
}

func TestEnsureBattlefieldFrontFace_MetadataSwapStillWins(t *testing.T) {
	// With BackFace metadata present the proper SwapToBackFace path
	// must keep doing the full identity swap (name + types + CMC); the
	// new arm must not interfere.
	c := &Card{
		Name:             "Agadeem's Awakening // Agadeem, the Undercrypt",
		Types:            []string{"sorcery", "//", "land"},
		TypeLine:         "sorcery // land",
		CMC:              3,
		BackFaceName:     "Agadeem, the Undercrypt",
		BackFaceTypes:    []string{"land"},
		BackFaceTypeLine: "land",
	}
	EnsureBattlefieldFrontFace(c)
	if c.Name != "Agadeem, the Undercrypt" {
		t.Errorf("Name = %q, want back-face name", c.Name)
	}
	if !reflect.DeepEqual(c.Types, []string{"land"}) {
		t.Errorf("Types = %v, want [land]", c.Types)
	}
}
