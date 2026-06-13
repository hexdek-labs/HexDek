package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// Integration regression: these doublers were INERT (their replacement
// effects were never registered at ETB). After wiring via the per_card ETB
// hook, entering the doubler and then adding counters / creating tokens must
// double.

// enterDoubler puts a doubler on the battlefield and fires the real ETB hook
// (which runs the per_card OnETB that registers its replacement).
func enterDoubler(t *testing.T, gs *gameengine.GameState, seat int, name string) *gameengine.Permanent {
	t.Helper()
	p := addPerm(gs, seat, name, "enchantment")
	gameengine.InvokeETBHook(gs, p)
	return p
}

func TestUnwiredDoubler_PrimalVigorDoublesCounters(t *testing.T) {
	gs := newGame(t, 2)
	enterDoubler(t, gs, 0, "Primal Vigor")

	target := addPerm(gs, 0, "Walking Ballista", "creature", "artifact")
	target.Card.BasePower, target.Card.BaseToughness = 0, 0

	placed, _ := gameengine.AddCountersToPermanent(gs, target, "+1/+1", 2, "src-pv", false)
	if placed != 4 {
		t.Fatalf("Primal Vigor should double 2 → 4 +1/+1 counters, got %d", placed)
	}
}

func TestUnwiredDoubler_BranchingEvolutionDoublesCounters(t *testing.T) {
	gs := newGame(t, 2)
	enterDoubler(t, gs, 0, "Branching Evolution")

	target := addPerm(gs, 0, "Hangarback", "creature", "artifact")
	target.Card.BasePower, target.Card.BaseToughness = 0, 0

	placed, _ := gameengine.AddCountersToPermanent(gs, target, "+1/+1", 3, "src-be", false)
	if placed != 6 {
		t.Fatalf("Branching Evolution should double 3 → 6 +1/+1 counters, got %d", placed)
	}
}

func TestUnwiredDoubler_ConclaveMentorDoublesCounters(t *testing.T) {
	gs := newGame(t, 2)
	enterDoubler(t, gs, 0, "Conclave Mentor")

	target := addPerm(gs, 0, "Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 2, 2

	placed, _ := gameengine.AddCountersToPermanent(gs, target, "+1/+1", 1, "src-cm", false)
	if placed != 2 {
		t.Fatalf("Conclave Mentor should double 1 → 2 +1/+1 counters, got %d", placed)
	}
}

func TestUnwiredDoubler_ParallelLivesDoublesTokens(t *testing.T) {
	gs := newGame(t, 2)
	pl := enterDoubler(t, gs, 0, "Parallel Lives")

	// Fire the would_create_token event for the controller — should double.
	n, _ := gameengine.FireCreateTokenEvent(gs, 0, 1, pl)
	if n != 2 {
		t.Fatalf("Parallel Lives should double 1 → 2 tokens, got %d", n)
	}
	// An opponent creating tokens is NOT doubled.
	n2, _ := gameengine.FireCreateTokenEvent(gs, 1, 3, pl)
	if n2 != 3 {
		t.Fatalf("Parallel Lives must not double an opponent's tokens, got %d", n2)
	}
}

func TestUnwiredDoubler_PrimalVigorDoublesTokens(t *testing.T) {
	gs := newGame(t, 2)
	pv := enterDoubler(t, gs, 0, "Primal Vigor")
	// Primal Vigor doubles tokens for ALL players (symmetric).
	n, _ := gameengine.FireCreateTokenEvent(gs, 0, 2, pv)
	if n != 4 {
		t.Fatalf("Primal Vigor should double 2 → 4 tokens, got %d", n)
	}
}

func TestUnwiredDoubler_NotDoubledWhenAbsent(t *testing.T) {
	gs := newGame(t, 2)
	// No doubler on the battlefield — counts pass through unchanged.
	target := addPerm(gs, 0, "Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 2, 2
	placed, _ := gameengine.AddCountersToPermanent(gs, target, "+1/+1", 2, "src-x", false)
	if placed != 2 {
		t.Fatalf("with no doubler, 2 counters should stay 2, got %d", placed)
	}
	n, _ := gameengine.FireCreateTokenEvent(gs, 0, 1, nil)
	if n != 1 {
		t.Fatalf("with no doubler, 1 token should stay 1, got %d", n)
	}
}
