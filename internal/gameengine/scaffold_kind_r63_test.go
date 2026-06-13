package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// scaffold_kind_r63_test.go — regression pins for the r63 systematic
// scaffold-KIND coverage (hex-dev-3, category: direct life/damage &
// monarch game-state effects). Each kind below was previously unhandled
// by resolveModificationEffect's switch (fell to the inert default that
// only logs a parser_gap). A representative card now produces its effect.

// become_monarch_typed — the typed-template variant of become_monarch,
// emitted for "you become the monarch" ETB/cast clauses (Custodi Lich,
// Palace Jailer, the Court enchantments). 37 cards.
func TestScaffoldKind_BecomeMonarchTyped(t *testing.T) {
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 1, "Custodi Lich", 2, 3, "creature")

	if IsMonarch(gs, 1) {
		t.Fatal("precondition: seat 1 should not start as monarch")
	}
	eff := &gameast.ModificationEffect{ModKind: "become_monarch_typed"}
	ResolveEffect(gs, src, eff)

	if !IsMonarch(gs, 1) {
		t.Error("become_monarch_typed: seat 1 (source controller) should be the monarch")
	}
}

// self_damage — "this <source> deals N damage to you" riders, dominantly
// the painlands (Shivan Reef, Ancient Tomb, Grand Coliseum). Args[0] is
// the amount; damage is dealt to the source's controller. 30 cards.
func TestScaffoldKind_SelfDamage(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	src := addBattlefield(gs, 0, "Ancient Tomb", 0, 0, "land")

	eff := &gameast.ModificationEffect{
		ModKind: "self_damage",
		Args:    []interface{}{2},
	}
	ResolveEffect(gs, src, eff)

	if gs.Seats[0].Life != 38 {
		t.Errorf("self_damage: controller life = %d, want 38 (-2 to you)", gs.Seats[0].Life)
	}
}

// self_damage with the default (no/zero arg) deals 1 — the painland norm.
func TestScaffoldKind_SelfDamage_DefaultsToOne(t *testing.T) {
	gs := newFixtureGame(t)
	gs.Seats[0].Life = 40
	src := addBattlefield(gs, 0, "Shivan Reef", 0, 0, "land")

	eff := &gameast.ModificationEffect{
		ModKind: "self_damage",
		Args:    []interface{}{1},
	}
	ResolveEffect(gs, src, eff)

	if gs.Seats[0].Life != 39 {
		t.Errorf("self_damage: controller life = %d, want 39 (-1)", gs.Seats[0].Life)
	}
}
