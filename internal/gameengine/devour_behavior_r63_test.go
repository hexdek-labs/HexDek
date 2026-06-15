package gameengine

import (
	"math/rand"
	"testing"
)

// devour_behavior_r63_test.go — behavioral regressions for Devour (CR §702.82).
// ApplyDevour/ApplyDevourTyped placed the enters-with +1/+1 counters with raw
// AddCounter, bypassing the §122.1g doubler chain. These pin the doubler-aware
// counter plus the per-creature multiplication, token handling, dies-triggers,
// and the zero-devour no-op.

func newDevourGame(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(82)), nil)
	gs.Active = 0
	return gs
}

func devourCreature(gs *GameState, seat int, name string) *Permanent {
	return addKWCombatBattlefield(gs, seat, name, 1, 1, "creature")
}

func devourSacEvents(gs *GameState) int {
	n := 0
	for _, ev := range gs.EventLog {
		if ev.Kind == "sacrifice" {
			n++
		}
	}
	return n
}

// (b) Enters with N +1/+1 counters PER creature sacrificed.
func TestDevour_CountersPerCreatureSacrificed(t *testing.T) {
	gs := newDevourGame(t)
	mycoloth := devourCreature(gs, 0, "Mycoloth")
	devourCreature(gs, 0, "Fodder1")
	devourCreature(gs, 0, "Fodder2")
	devourCreature(gs, 0, "Fodder3")

	ApplyDevour(gs, mycoloth, 3) // heuristic sacrifices 2 → 2 * 3 = 6 counters

	if mycoloth.Counters["+1/+1"] != 6 {
		t.Errorf("(b) devour 3 with 2 sacrificed should give 6 counters, got %d", mycoloth.Counters["+1/+1"])
	}
}

// (c) The entering counters respect +1/+1 doublers (Doubling Season).
func TestDevour_CountersRespectDoublers(t *testing.T) {
	gs := newDevourGame(t)
	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)
	beast := devourCreature(gs, 0, "Caller of the Untamed")
	devourCreature(gs, 0, "Fodder1")
	devourCreature(gs, 0, "Fodder2")
	devourCreature(gs, 0, "Fodder3")

	ApplyDevour(gs, beast, 1) // sacrifices 2 → base 2 → Doubling Season → 4

	if beast.Counters["+1/+1"] != 4 {
		t.Errorf("(c) devour counters must double under Doubling Season (2→4), got %d", beast.Counters["+1/+1"])
	}
}

// (e)+(g) Tokens count toward devour and cease; every devoured creature's
// sacrifice routes through the canonical path (dies/LTB fire).
func TestDevour_TokensCountAndDiesTriggersFire(t *testing.T) {
	gs := newDevourGame(t)
	beast := devourCreature(gs, 0, "Thunder-Thrash Elder")
	tok := devourCreature(gs, 0, "Saproling Token")
	tok.Card.Types = append(tok.Card.Types, "token")
	devourCreature(gs, 0, "Fodder2")
	devourCreature(gs, 0, "Fodder3")

	ApplyDevour(gs, beast, 1) // sacrifices 2 (incl. the token if first in order)

	// 2 creatures sacrificed → 2 canonical sacrifice events.
	if devourSacEvents(gs) != 2 {
		t.Errorf("(g) devour should emit 2 canonical sacrifice events, got %d", devourSacEvents(gs))
	}
	// The token was sacrificed (no longer on the battlefield).
	stillThere := false
	for _, p := range gs.Seats[0].Battlefield {
		if p == tok {
			stillThere = true
		}
	}
	if stillThere {
		t.Error("(e) the sacrificed token must leave the battlefield")
	}
	if beast.Counters["+1/+1"] != 2 {
		t.Errorf("counters should be 2 (devour 1 × 2 sacrificed), got %d", beast.Counters["+1/+1"])
	}
}

// (f) Devouring zero creatures is legal — enters with 0 counters, no crash.
func TestDevour_ZeroIsLegal(t *testing.T) {
	gs := newDevourGame(t)
	lonely := devourCreature(gs, 0, "Lone Mycoloth") // no other creatures to devour

	ApplyDevour(gs, lonely, 5)

	if lonely.Counters["+1/+1"] != 0 {
		t.Errorf("(f) devouring zero creatures should give 0 counters, got %d", lonely.Counters["+1/+1"])
	}
}

// (b/c) ApplyDevourTyped (e.g. "Devour land N") routes its counters through the
// same doubler-aware path.
func TestDevourTyped_CountersRespectDoublers(t *testing.T) {
	gs := newDevourGame(t)
	ds := addKWCombatBattlefield(gs, 0, "Doubling Season", 0, 0, "enchantment")
	RegisterDoublingSeason(gs, ds)
	worldsire := devourCreature(gs, 0, "Famished Worldsire")
	addKWCombatBattlefield(gs, 0, "Forest1", 0, 0, "land")
	addKWCombatBattlefield(gs, 0, "Forest2", 0, 0, "land")
	addKWCombatBattlefield(gs, 0, "Forest3", 0, 0, "land")
	addKWCombatBattlefield(gs, 0, "Forest4", 0, 0, "land")

	ApplyDevourTyped(gs, worldsire, 1, "land") // keeps 2 lands → sacrifices 2 → base 2 → doubled 4

	if worldsire.Counters["+1/+1"] != 4 {
		t.Errorf("(c) ApplyDevourTyped counters must double under Doubling Season, got %d", worldsire.Counters["+1/+1"])
	}
}
