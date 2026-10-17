package gameengine

import (
	"strings"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r157 — the three per-Card derived caches (oracle text, type line,
// produced-colors mask) were each keyed on a "have we computed this"
// bool. A DFC transform reassigns Card.AST and Card.TypeLine in place,
// and no transform site cleared those flags, so every reader kept
// seeing the FRONT face for the rest of the game.
//
// Exposure when this was found: 657 of 1,727 live decks (38.0%) carry
// at least one transform card, 1,080 slots in total.

// r157Wolf builds a battlefield permanent set up as a transforming DFC
// whose two faces differ in oracle text AND type line.
func r157Wolf(t *testing.T, gs *GameState) (*Permanent, *gameast.CardAST, *gameast.CardAST) {
	t.Helper()
	front := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "Front shape: creatures you control get +1/+1."},
	}}
	back := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "Back shape: whenever you draw a card, each opponent loses 1 life."},
	}}
	p := addBattlefield(gs, 0, "Probe Wolf", 2, 2, "creature")
	p.Card.AST = front
	p.FrontFaceAST = front
	p.BackFaceAST = back
	p.FrontFaceName = "Probe Wolf"
	p.BackFaceName = "Probe Wolf, Howling"
	p.Card.TypeLine = "Creature — Human Werewolf"
	p.Card.BackFaceTypeLine = "Creature — Werewolf"
	return p, front, back
}

// Direction 1: the front face is served before the transform.
// Direction 2: the BACK face is served after it.
func TestR157_OracleTextFollowsTheTransform(t *testing.T) {
	gs := newFixtureGame(t)
	p, _, back := r157Wolf(t, gs)

	before := OracleTextLower(p.Card)
	if !strings.Contains(before, "front shape") {
		t.Fatalf("precondition: want the front face before transform, got %q", before)
	}

	if !TransformPermanent(gs, p, "r157") {
		t.Fatalf("TransformPermanent refused")
	}
	if p.Card.AST != back {
		t.Fatalf("precondition: transform did not swap the AST")
	}

	after := OracleTextLower(p.Card)
	if !strings.Contains(after, "back shape") {
		t.Errorf("STALE: oracle text after transform = %q, want the back face", after)
	}
	if strings.Contains(after, "front shape") {
		t.Errorf("STALE: oracle text still carries the front face: %q", after)
	}
}

// The type line is reassigned by a different code path (dfc.go:489 /
// stack.go:2676) than the AST, so it gets its own direction pair
// driven directly rather than through TransformPermanent.
func TestR157_TypeLineLowerFollowsAReassignment(t *testing.T) {
	c := &Card{TypeLine: "Enchantment"}

	if got := TypeLineLower(c); got != "enchantment" {
		t.Fatalf("precondition: got %q", got)
	}
	c.TypeLine = "Land"
	if got := TypeLineLower(c); got != "land" {
		t.Errorf("STALE: type line after reassignment = %q, want \"land\"", got)
	}
}

// Search for Azcanta is the real shape: the FRONT face is an
// enchantment that produces no mana, the BACK face is a land that taps
// for {U}. The derived mask must not survive the transform.
func TestR157_ProducedColorsMaskFollowsTheTransform(t *testing.T) {
	front := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "At the beginning of your upkeep, look at the top card of your library."},
	}}
	back := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Activated{Raw: "{T}: Add {U}."},
	}}
	c := &Card{Name: "Search for Azcanta", AST: front, TypeLine: "Enchantment"}

	if mask := landProducedColorsMask(c); mask != 0 {
		t.Fatalf("precondition: front face should produce nothing, got mask %d", mask)
	}

	// The transform, as dfc.go performs it.
	c.AST = back
	c.TypeLine = "Land"

	mask := landProducedColorsMask(c)
	if mask&colorBitMaskU == 0 {
		t.Errorf("STALE: back face is a land that taps for {U}, got mask %d", mask)
	}
}

// A cold read must not serve a zero-value cache. A freshly built Card
// has a nil oracleTextAST; so does a Card with a nil AST. The nil-AST
// guard must win, and a real AST must never match the zero value.
func TestR157_ZeroValueNeverServesACache(t *testing.T) {
	bare := &Card{}
	if got := OracleTextLower(bare); got != "" {
		t.Errorf("nil AST should yield \"\", got %q", got)
	}

	ast := &gameast.CardAST{Abilities: []gameast.Ability{
		&gameast.Static{Raw: "Real text here."},
	}}
	c := &Card{AST: ast}
	// OracleTextCache is deliberately pre-poisoned: a zero-value
	// oracleTextAST must not be treated as "matches".
	c.OracleTextCache = "poison"
	if got := OracleTextLower(c); !strings.Contains(got, "real text") {
		t.Errorf("cold read served the poisoned cache: %q", got)
	}
}

// Derived gate. The defect was a cache keyed on "computed yet" rather
// than on the value it was derived from, and the fix is only durable
// if the shape does not come back. Any *Card cache paired with a bare
// readiness bool is the shape that broke; this asserts the three known
// ones are keyed on a source instead.
func TestR157_DerivedGate_CachesAreKeyedOnTheirSource(t *testing.T) {
	ast1 := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Static{Raw: "One."}}}
	ast2 := &gameast.CardAST{Abilities: []gameast.Ability{&gameast.Static{Raw: "Two."}}}

	c := &Card{AST: ast1, TypeLine: "Artifact"}
	_ = OracleTextLower(c)
	_ = TypeLineLower(c)
	_ = landProducedColorsMask(c)

	if !c.ProducedColorsValid() {
		t.Fatalf("precondition: mask should be valid immediately after computing it")
	}

	// Swapping either source must invalidate the derived mask.
	c.AST = ast2
	if c.ProducedColorsValid() {
		t.Errorf("mask stayed valid after the AST was replaced")
	}
	c.AST = ast1
	c.TypeLine = "Land"
	if c.ProducedColorsValid() {
		t.Errorf("mask stayed valid after the type line was replaced")
	}
}
