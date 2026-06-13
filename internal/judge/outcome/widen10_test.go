package outcome

// Phase-10 unit suite — GainLife/LoseLife player-target symmetry. GainLife
// previously modeled only the self target; it now mirrors LoseLife's
// each-opponent / each-player / "opponent each" vocabulary, plus a
// policy-targeted "target player" disjunction.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestWiden10_GainLifeTargets(t *testing.T) {
	mustPass(t, "gain-each-opponent", &gameast.GainLife{Amount: *gameast.NumInt(3),
		Target: gameast.Filter{Base: "each_opponent", Quantifier: "each"}})
	mustPass(t, "gain-opponent-each", &gameast.GainLife{Amount: *gameast.NumInt(3),
		Target: gameast.Filter{Base: "opponent", Quantifier: "each"}})
	mustPass(t, "gain-each-player", &gameast.GainLife{Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "each_player", Quantifier: "each"}})
	mustPass(t, "gain-player-each", &gameast.GainLife{Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "player", Quantifier: "each"}})
	// Self still works.
	mustPass(t, "gain-self", &gameast.GainLife{Amount: *gameast.NumInt(5),
		Target: gameast.Filter{Base: "self", Quantifier: "one"}})
	// Policy-targeted "target player gains" → two-seat disjunction.
	mustPass(t, "gain-target-player", &gameast.GainLife{Amount: *gameast.NumInt(4),
		Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true}})
}

func TestWiden10_LoseLifeTargetsWidened(t *testing.T) {
	// "each_player" is plural by base — both seats even with q=one.
	mustPass(t, "lose-each-player-one", &gameast.LoseLife{Amount: *gameast.NumInt(3),
		Target: gameast.Filter{Base: "each_player", Quantifier: "one"}})
	mustPass(t, "lose-opponent-each", &gameast.LoseLife{Amount: *gameast.NumInt(3),
		Target: gameast.Filter{Base: "opponent", Quantifier: "each"}})
}

func TestWiden10_LifeUnmodelableStaysOut(t *testing.T) {
	// "var" amount — the parser's unstructured-scaling escape hatch
	// (engine reads an unset src.Flags["var"] → 0): out of scope.
	mustSkip(t, "gain-var", &gameast.GainLife{
		Amount: gameast.NumberOrRef{IsStr: true, Str: "var"},
		Target: gameast.Filter{Base: "self", Quantifier: "one"}})
	// Contextual single-player referent: unresolvable from the AST alone.
	mustSkip(t, "lose-that-player", &gameast.LoseLife{Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "that_player", Quantifier: "one"}})
}
