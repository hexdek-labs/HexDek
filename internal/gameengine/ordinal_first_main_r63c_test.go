package gameengine

// r63c PROGRESSION phase-3c finding: the parser emits Event
// "beginning_of_ordinal_step" with an EMPTY Phase field for "at the
// beginning of your first main phase," triggers, so triggerMatchesPhaseStep
// dropped EVERY such trigger — Coalition Relic, Altar of Shadows, Abstract
// Paintmage, Four Knocks et al. had their first-main mana / draw / counter
// abilities silently inert. The chaos runner already fires
// FirePhaseTriggers(gs,"precombat_main","main"); the raw-aware matcher
// recovers the ordinal from the oracle wording and gates it to the
// matching main-phase boundary.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func firstMainDrawBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "beginning_of_ordinal_step"},
		Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "First Main Bearer", 2, 2, "enchantment")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestOrdinalFirstMain_FiresAtPrecombatMain(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	firstMainDrawBearer(gs, 0, "at the beginning of your first main phase, draw a card")

	gs.Active = 0
	before := len(gs.Seats[0].Hand)
	FirePhaseTriggers(gs, "precombat_main", "main")
	if got := len(gs.Seats[0].Hand) - before; got != 1 {
		t.Fatalf("first-main trigger must fire at precombat main: drew %d cards, want 1", got)
	}
}

func TestOrdinalFirstMain_ControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	firstMainDrawBearer(gs, 0, "at the beginning of your first main phase, draw a card")

	// Opponent's first main phase — "your" scope must stay silent.
	gs.Active = 1
	before := len(gs.Seats[0].Hand)
	FirePhaseTriggers(gs, "precombat_main", "main")
	if got := len(gs.Seats[0].Hand) - before; got != 0 {
		t.Fatalf("your-first-main must not fire on the opponent's main phase: drew %d", got)
	}
}

func TestOrdinalSecondMain_DoesNotFireAtPrecombat(t *testing.T) {
	gs := newFixtureGame(t)
	addLibrary(gs, 0, "Card A", "Card B")
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "beginning_of_ordinal_step"},
		Effect:  &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     "at the beginning of your second main phase, draw a card",
	}
	p := addBattlefield(gs, 0, "Second Main Bearer", 2, 2, "enchantment")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}

	gs.Active = 0
	before := len(gs.Seats[0].Hand)
	FirePhaseTriggers(gs, "precombat_main", "main")
	if got := len(gs.Seats[0].Hand) - before; got != 0 {
		t.Fatalf("second-main trigger must NOT fire at precombat (first) main: drew %d", got)
	}
	// It DOES fire at postcombat main.
	FirePhaseTriggers(gs, "postcombat_main", "main")
	if got := len(gs.Seats[0].Hand) - before; got != 1 {
		t.Fatalf("second-main trigger must fire at postcombat main: drew %d, want 1", got)
	}
}
