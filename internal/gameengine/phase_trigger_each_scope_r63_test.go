package gameengine

// r63 PROGRESSION phase-2 finding: the parser emits Controller=None for
// ALL phase triggers, and triggerControllerMatches defaulted "" to
// "your upkeep only" — silently disabling the each-player scope for
// every "at the beginning of each upkeep/end step" card (Baleful Force:
// "at the beginning of each upkeep, you draw a card and you lose 1
// life" fired once per round instead of once per upkeep). The raw-aware
// matcher recovers the scope from the oracle wording until the parser
// carries it.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func eachUpkeepBearer(gs *GameState, seat int, raw string) *Permanent {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "phase", Phase: "upkeep"},
		Effect:  &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}},
		Raw:     raw,
	}
	p := addBattlefield(gs, seat, "Phase Bearer", 4, 4, "creature")
	p.Card.AST = &gameast.CardAST{Abilities: []gameast.Ability{tr}}
	return p
}

func TestPhaseTrigger_EachUpkeepFiresOnEveryUpkeep(t *testing.T) {
	gs := newFixtureGame(t)
	eachUpkeepBearer(gs, 0, "at the beginning of each upkeep, you gain 1 life")

	gs.Active = 0
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 21 {
		t.Fatalf("each-upkeep must fire on controller's upkeep: life=%d", gs.Seats[0].Life)
	}
	gs.Active = 1 // opponent's upkeep
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 22 {
		t.Fatalf("each-upkeep must ALSO fire on the opponent's upkeep (Baleful Force class): life=%d", gs.Seats[0].Life)
	}
}

func TestPhaseTrigger_YourUpkeepStaysControllerGated(t *testing.T) {
	gs := newFixtureGame(t)
	eachUpkeepBearer(gs, 0, "at the beginning of your upkeep, you gain 1 life")

	gs.Active = 1 // opponent's upkeep — must stay silent
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 20 {
		t.Fatalf("your-upkeep must not fire on the opponent's upkeep: life=%d", gs.Seats[0].Life)
	}
	gs.Active = 0
	FirePhaseTriggers(gs, "beginning", "upkeep")
	if gs.Seats[0].Life != 21 {
		t.Fatalf("your-upkeep must fire on the controller's upkeep: life=%d", gs.Seats[0].Life)
	}
}
