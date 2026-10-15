package gameengine

import (
	"math/rand"
	"testing"
)

// r120 — CR §704.5 splits defense-0 battles into two rules and the engine
// had them merged under one:
//
//	704.5v  If a SIEGE battle has defense 0 AND it isn't the source of an
//	        ability that has triggered but not yet left the stack, it's put
//	        into its owner's graveyard.
//	704.5w  If a NON-SIEGE battle has defense 0, it's put into its owner's
//	        graveyard.
//
// The triggered-ability clause is the entire difference, and it belongs to
// the Siege arm only. The merged function applied it to every battle, so a
// non-Siege battle at defense 0 that was the source of a trigger still on
// the stack stayed on the battlefield.
//
// Reachable, not theoretical: the corpus holds two non-Siege battles,
// Occupation of Kulrath and Occupation of Llanowar (Battle — Control
// Point). That same CR split is why every subrule letter from here on was
// one behind.

func r120Game(t *testing.T) *GameState {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(120)), nil)
	gs.Active = 0
	return gs
}

// r120Battle places a battle on seat 0 at the given defense. Battle types
// live in Card.Types beside the card types — the runtime Card has no
// Subtypes field.
func r120Battle(gs *GameState, name string, defense int, siege bool) *Permanent {
	types := []string{"battle"}
	if siege {
		types = append(types, "siege")
	}
	p := addBattlefield(gs, 0, name, 0, 0, types...)
	p.Counters["defense"] = defense
	p.Flags["has_protector"] = 1
	return p
}

func r120OnBattlefield(gs *GameState, p *Permanent) bool {
	for _, q := range gs.Seats[0].Battlefield {
		if q == p {
			return true
		}
	}
	return false
}

// The fix, both directions, with the Siege as its own control in the SAME
// state: identical defense, identical stack entry, opposite outcome.
func TestBattleDefenseSplitR120_StackClauseIsSiegeOnly(t *testing.T) {
	gs := r120Game(t)
	siege := r120Battle(gs, "R120 Invasion", 0, true)
	control := r120Battle(gs, "R120 Occupation", 0, false)

	// Both are the source of a triggered ability still on the stack.
	gs.Stack = append(gs.Stack,
		&StackItem{ID: 1, Controller: 0, Source: siege},
		&StackItem{ID: 2, Controller: 0, Source: control},
	)

	StateBasedActions(gs)

	if !r120OnBattlefield(gs, siege) {
		t.Error("§704.5v: a SIEGE at defense 0 that is the source of an ability " +
			"on the stack must stay on the battlefield")
	}
	if r120OnBattlefield(gs, control) {
		t.Error("§704.5w: a NON-SIEGE at defense 0 goes to the graveyard even " +
			"while it is the source of an ability on the stack — that rule has " +
			"no triggered-ability clause")
	}
}

// With nothing on the stack both arms behave the same, which is what makes
// the test above about the clause rather than about battles in general.
func TestBattleDefenseSplitR120_BothDieWithEmptyStack(t *testing.T) {
	gs := r120Game(t)
	siege := r120Battle(gs, "R120 Invasion B", 0, true)
	control := r120Battle(gs, "R120 Occupation B", 0, false)

	StateBasedActions(gs)

	if r120OnBattlefield(gs, siege) {
		t.Error("§704.5v: a Siege at defense 0 with an empty stack must die")
	}
	if r120OnBattlefield(gs, control) {
		t.Error("§704.5w: a non-Siege at defense 0 with an empty stack must die")
	}
}

// And neither arm may fire above defense 0 — the guard both share.
func TestBattleDefenseSplitR120_PositiveDefenseSurvives(t *testing.T) {
	gs := r120Game(t)
	siege := r120Battle(gs, "R120 Invasion C", 3, true)
	control := r120Battle(gs, "R120 Occupation C", 3, false)

	StateBasedActions(gs)

	if !r120OnBattlefield(gs, siege) {
		t.Error("a Siege at defense 3 must survive")
	}
	if !r120OnBattlefield(gs, control) {
		t.Error("a non-Siege at defense 3 must survive")
	}
}

// permIsSiege reads the battle type out of Card.Types; pin that, since the
// absence of a Subtypes field is exactly the kind of thing that gets
// "fixed" later into a field that does not exist.
func TestBattleDefenseSplitR120_SiegeDetection(t *testing.T) {
	gs := r120Game(t)
	if !permIsSiege(r120Battle(gs, "R120 Siege D", 5, true)) {
		t.Error("a battle carrying the siege type must read as a Siege")
	}
	if permIsSiege(r120Battle(gs, "R120 Control D", 5, false)) {
		t.Error("a battle without the siege type must not read as a Siege")
	}
	if permIsSiege(nil) {
		t.Error("nil must not read as a Siege")
	}
}
