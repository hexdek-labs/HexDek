package gameengine

// r63 PROGRESSION saturation finding: bare "whenever a/another creature you
// control dies," triggers (the parser drops the actor phrase) were silent —
// the observer death walk treated nil-actor creature_dies as a self-trigger
// (skipped) and the actor-keyed matcher returned false on nil Actor. The
// raw-based observer-death dispatch (observerDeathMatchesByRaw) now matches
// them; these tests pin the real DestroyPermanent chokepoint.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func allyDiesBearer(gs *GameState, seat int, raw, event string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: event},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Aristocrat", 2, 2, "creature")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestAllyDies_FiresOnControlledCreatureDeath(t *testing.T) {
	gs := newFixtureGame(t)
	allyDiesBearer(gs, 0, "whenever a creature you control dies, you gain 1 life", "creature_dies")
	ally := addBattlefield(gs, 0, "Token Soldier", 1, 1, "creature")

	before := gs.Seats[0].Life
	DestroyPermanent(gs, ally, nil)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("ally-dies payoff must fire on a controlled creature's death: gained %d, want 1", got)
	}
}

func TestAllyDies_ControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	allyDiesBearer(gs, 0, "whenever a creature you control dies, you gain 1 life", "creature_dies")
	oppCreature := addBattlefield(gs, 1, "Opp Soldier", 1, 1, "creature")

	before := gs.Seats[0].Life
	DestroyPermanent(gs, oppCreature, nil)
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("you-control ally-dies must not fire on an opponent's creature death: gained %d", got)
	}
}

func TestAllyDies_AnyControllerFiresForOpponent(t *testing.T) {
	gs := newFixtureGame(t)
	// Blood Artist form: "whenever a creature dies" — no controller gate.
	allyDiesBearer(gs, 0, "whenever a creature dies, you gain 1 life", "creature_dies")
	oppCreature := addBattlefield(gs, 1, "Opp Soldier", 1, 1, "creature")

	before := gs.Seats[0].Life
	DestroyPermanent(gs, oppCreature, nil)
	if got := gs.Seats[0].Life - before; got != 1 {
		t.Fatalf("'a creature dies' must fire for any creature's death: gained %d, want 1", got)
	}
}

func TestAllyDies_AnotherExcludesSelf(t *testing.T) {
	gs := newFixtureGame(t)
	// "another creature you control" must not count the bearer's own death.
	bearer := allyDiesBearer(gs, 0, "whenever another creature you control dies, you gain 1 life", "another_creature_dies")

	before := gs.Seats[0].Life
	DestroyPermanent(gs, bearer, nil) // the bearer itself dies
	if got := gs.Seats[0].Life - before; got != 0 {
		t.Fatalf("'another creature you control' must not fire on the bearer's own death: gained %d", got)
	}
}
