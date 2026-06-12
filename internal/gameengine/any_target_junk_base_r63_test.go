package gameengine

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// r63 — any-target filters with parser-conjunction bases.
//
// For compound damage clauses the parser leaves the conjunction word in
// Filter.Base: "deals 4 damage to any target and 2 damage to you"
// (Psionic Blast, Orcish Cannonade, Seismic Wave, Reveka) emits
// {base:"and", quantifier:"any"}, and "to any target unless that
// player pays {2}" (Rhystic Lightning) emits {base:"unless",
// quantifier:"any"}. Pre-r63 pickPermanentTarget's any-target gate
// didn't recognize the junk bases, the type matcher matched nothing,
// and the spell resolved with no target — the damage never happened.

func anyTargetDamageFixture(t *testing.T, base string) (*GameState, int) {
	t.Helper()
	gs := newFixtureGame(t)
	src := addBattlefield(gs, 0, "Psionic Blast", 0, 0, "instant")
	before := gs.Seats[1].Life

	ResolveEffect(gs, src, &gameast.Damage{
		Amount: gameast.NumberOrRef{IsInt: true, Int: 4},
		Target: gameast.Filter{Base: base, Quantifier: "any", Targeted: true},
	})
	return gs, before
}

func TestAnyTargetDamage_AndBase_HitsOpponent(t *testing.T) {
	gs, before := anyTargetDamageFixture(t, "and")
	if got := gs.Seats[1].Life; got != before-4 {
		t.Errorf("expected opponent life %d after 4 damage to 'any target' (base=and), got %d", before-4, got)
	}
}

func TestAnyTargetDamage_UnlessBase_HitsOpponent(t *testing.T) {
	gs, before := anyTargetDamageFixture(t, "unless")
	if got := gs.Seats[1].Life; got != before-4 {
		t.Errorf("expected opponent life %d after 4 damage to 'any target' (base=unless), got %d", before-4, got)
	}
}
