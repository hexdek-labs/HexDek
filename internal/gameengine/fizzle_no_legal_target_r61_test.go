package gameengine

import (
	"math/rand"
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// CR §608.2b legal-target / fizzle hole (r61 PR-3).
//
// The engine picks targets LAZILY at resolution (StackItem pushed with empty
// Targets; the real target chosen inside effect handlers via PickTarget). With
// item.Targets empty, the §608.2b fizzle check was SKIPPED, and effect handlers
// MANUFACTURED a target (resolve.go "Default to opponent" → Opponents(...)[0]).
// So a targeted "target opponent loses life" (Vito-style) with NO legal target
// (all opponents dead) resolved against a fake target instead of fizzling.
//
// These tests pin the resolution-time fizzle gate in ResolveStackTop.

// TestFizzle_TargetOpponentLosesLife_NoLegalTarget pins the Vito case: a
// targeted "target opponent loses life" spell resolving when the controller is
// the only living player must FIZZLE — no life loss to the controller, and the
// spell card moves stack → graveyard per §608.2b.
func TestFizzle_TargetOpponentLosesLife_NoLegalTarget(t *testing.T) {
	rng := rand.New(rand.NewSource(61))
	gs := NewGameState(4, rng, nil)

	// Controller is seat 0; every opponent is already dead.
	for i := 1; i < len(gs.Seats); i++ {
		gs.Seats[i].Lost = true
		gs.Seats[i].LeftGame = true
	}
	controllerLifeBefore := gs.Seats[0].Life

	// A "target opponent loses 3 life" instant cast by seat 0.
	card := &Card{Name: "Drain the Lone", Owner: 0, Types: []string{"instant"}}
	item := &StackItem{
		Controller: 0,
		Card:       card,
		Kind:       "spell",
		Effect: &gameast.LoseLife{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(), // Targeted, Base "opponent"
		},
	}
	gs.Stack = append(gs.Stack, item)

	ResolveStackTop(gs)

	// Controller must NOT have lost life (no manufactured self/opponent target).
	if gs.Seats[0].Life != controllerLifeBefore {
		t.Fatalf("controller life changed on a fizzle: before=%d after=%d (effect manufactured a target instead of fizzling)",
			controllerLifeBefore, gs.Seats[0].Life)
	}

	// The fizzled spell must have left the stack and gone to its owner's graveyard.
	if len(gs.Stack) != 0 {
		t.Fatalf("fizzled spell still on stack: %d items", len(gs.Stack))
	}
	foundInGrave := false
	for _, c := range gs.Seats[0].Graveyard {
		if c == card {
			foundInGrave = true
			break
		}
	}
	if !foundInGrave {
		t.Fatalf("fizzled spell card did not move to owner's graveyard per §608.2b")
	}
}

// TestFizzle_TargetOpponentLosesLife_LegalTargetResolves confirms the gate is
// NOT over-aggressive: the SAME targeted effect WITH a legal opponent resolves
// normally and drains that opponent.
func TestFizzle_TargetOpponentLosesLife_LegalTargetResolves(t *testing.T) {
	rng := rand.New(rand.NewSource(62))
	gs := NewGameState(4, rng, nil)

	// Seat 0 is the controller; seat 1 is a live opponent. Kill seats 2 and 3
	// so PickTarget's "lowest-life opponent" lands on seat 1 deterministically.
	gs.Seats[2].Lost = true
	gs.Seats[2].LeftGame = true
	gs.Seats[3].Lost = true
	gs.Seats[3].LeftGame = true
	oppLifeBefore := gs.Seats[1].Life
	controllerLifeBefore := gs.Seats[0].Life

	card := &Card{Name: "Drain the Living", Owner: 0, Types: []string{"instant"}}
	item := &StackItem{
		Controller: 0,
		Card:       card,
		Kind:       "spell",
		Effect: &gameast.LoseLife{
			Amount: *gameast.NumInt(3),
			Target: gameast.TargetOpponent(),
		},
	}
	gs.Stack = append(gs.Stack, item)

	ResolveStackTop(gs)

	if gs.Seats[1].Life != oppLifeBefore-3 {
		t.Fatalf("legal-target drain did not resolve: opp life before=%d after=%d (want %d)",
			oppLifeBefore, gs.Seats[1].Life, oppLifeBefore-3)
	}
	if gs.Seats[0].Life != controllerLifeBefore {
		t.Fatalf("controller lost life on a legal-target drain: before=%d after=%d",
			controllerLifeBefore, gs.Seats[0].Life)
	}
}

// TestFizzle_EachOpponentLosesLife_NotFizzled confirms an UNTARGETED fan-out
// effect ("each opponent loses life") is never fizzled by the gate — it
// legitimately resolves (against whatever opponents exist) even though it has
// no single required target.
func TestFizzle_EachOpponentLosesLife_NotFizzled(t *testing.T) {
	rng := rand.New(rand.NewSource(63))
	gs := NewGameState(2, rng, nil)

	oppLifeBefore := gs.Seats[1].Life

	card := &Card{Name: "Group Drain", Owner: 0, Types: []string{"sorcery"}}
	item := &StackItem{
		Controller: 0,
		Card:       card,
		Kind:       "spell",
		Effect: &gameast.LoseLife{
			Amount: *gameast.NumInt(2),
			Target: gameast.EachOpponent(), // untargeted fan-out
		},
	}
	gs.Stack = append(gs.Stack, item)

	ResolveStackTop(gs)

	if gs.Seats[1].Life != oppLifeBefore-2 {
		t.Fatalf("each-opponent drain was wrongly fizzled or mis-resolved: opp before=%d after=%d (want %d)",
			oppLifeBefore, gs.Seats[1].Life, oppLifeBefore-2)
	}
}

// TestRequiredTargetFilter_Conservatism pins the predicate shape: only
// Targeted single/fixed-multi filters are reported as required; up_to_n / any /
// each / untargeted are NOT (they legally resolve with zero targets).
func TestRequiredTargetFilter_Conservatism(t *testing.T) {
	cases := []struct {
		name   string
		filter gameast.Filter
		want   bool
	}{
		{"target opponent", gameast.Filter{Base: "opponent", Targeted: true}, true},
		{"target creature", gameast.Filter{Base: "creature", Targeted: true}, true},
		{"two target creatures", gameast.Filter{Base: "creature", Targeted: true, Quantifier: "n"}, true},
		{"up to N target", gameast.Filter{Base: "creature", Targeted: true, Quantifier: "up_to_n"}, false},
		{"any number of target", gameast.Filter{Base: "creature", Targeted: true, Quantifier: "any"}, false},
		{"each opponent", gameast.Filter{Base: "opponent", Targeted: false, Quantifier: "each"}, false},
		{"untargeted a creature", gameast.Filter{Base: "creature", Targeted: false}, false},
	}
	for _, tc := range cases {
		if got := isRequiredSingleTargetFilter(tc.filter); got != tc.want {
			t.Errorf("%s: isRequiredSingleTargetFilter = %v, want %v", tc.name, got, tc.want)
		}
	}

	// requiredTargetFilter on a LoseLife{target opponent} item must report it.
	item := &StackItem{Effect: &gameast.LoseLife{Target: gameast.TargetOpponent()}}
	if _, ok := requiredTargetFilter(item); !ok {
		t.Errorf("requiredTargetFilter missed a LoseLife{target opponent}")
	}
	// A wrapper (Sequence) must NOT be reported as a single required target.
	wrapItem := &StackItem{Effect: &gameast.Sequence{Items: []gameast.Effect{
		gameast.LoseLife{Target: gameast.TargetOpponent()},
	}}}
	if _, ok := requiredTargetFilter(wrapItem); ok {
		t.Errorf("requiredTargetFilter wrongly reported a Sequence wrapper as a single required target")
	}
}
