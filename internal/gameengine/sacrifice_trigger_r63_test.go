package gameengine

// r63 PROGRESSION saturation finding: sacrificePermanentImpl fired only the
// per_card registry (creature_sacrificed / permanent_sacrificed / …) — no
// walk consulted the AST, so "whenever you sacrifice a <filter>," payoffs
// were silent unless the card had a bespoke handler. FireSacrificeASTTriggers
// is now driven from sacrificePermanentImpl; these tests pin the real
// chokepoint.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func sacPayoffBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "sacrifice_filtered"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Sac Payoff", 2, 2, "enchantment")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestSacrifice_FiresThroughSacrificePermanent(t *testing.T) {
	gs := newFixtureGame(t)
	sacPayoffBearer(gs, 0, "whenever you sacrifice a creature, you gain 1 life")
	victim := addBattlefield(gs, 0, "Token Goat", 0, 1, "creature")

	before := gs.Seats[0].Life
	SacrificePermanent(gs, victim, "test")
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("you-sacrifice payoff must fire through SacrificePermanent: gained %d life, want 1", got)
	}
}

func TestSacrifice_ControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	sacPayoffBearer(gs, 0, "whenever you sacrifice a creature, you gain 1 life")
	oppVictim := addBattlefield(gs, 1, "Opp Goat", 0, 1, "creature")

	before := gs.Seats[0].Life
	SacrificePermanent(gs, oppVictim, "test")
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("you-sacrifice payoff must not fire on the opponent's sacrifice: gained %d", got)
	}
}

func TestSacrifice_FilterTypeGate(t *testing.T) {
	gs := newFixtureGame(t)
	sacPayoffBearer(gs, 0, "whenever you sacrifice an artifact, you gain 1 life")
	// Sacrificing a CREATURE must not satisfy the "artifact" filter.
	victim := addBattlefield(gs, 0, "Token Goat", 0, 1, "creature")

	before := gs.Seats[0].Life
	SacrificePermanent(gs, victim, "test")
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("sacrifice-artifact payoff must not fire on a creature sacrifice: gained %d", got)
	}
}
