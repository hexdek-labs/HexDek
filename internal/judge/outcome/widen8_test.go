package outcome

// Phase-8 unit suite — structured ModificationEffect kinds, each
// exercised end-to-end through RunEffect against the live engine, plus
// the {X}/var skip-path pins (those amounts resolve against a harness
// convention, not a clean rules truth, so the band stays literal-only).

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func mod(kind string, args ...interface{}) *gameast.ModificationEffect {
	return &gameast.ModificationEffect{ModKind: kind, Args: args}
}

func TestWiden8_Monstrosity(t *testing.T) {
	// Source is a non-monstrous 4/4 creature → +N +1/+1 counters / +N P/T.
	mustPass(t, "monstrosity-3", mod("monstrosity", 3))
	mustPass(t, "monstrosity-float", mod("monstrosity", float64(4)))
	// monstrosity X: the engine ignores the harness's X convention
	// (asInt fails on "x"), and there is no clean rules truth to pin —
	// out of scope, never guessed.
	mustSkip(t, "monstrosity-x", mod("monstrosity", "x"))
}

func TestWiden8_Investigate(t *testing.T) {
	// N Clue tokens (0/0 noncreature artifacts) → battlefield +N only.
	mustPass(t, "investigate-1", mod("investigate", 1))
	mustPass(t, "investigate-twice", mod("investigate", 2))
	// "var" ("investigate that many times") is genuinely contextual —
	// unresolvable from the AST alone.
	mustSkip(t, "investigate-var", mod("investigate", "var"))
}

func TestWiden8_Incubate(t *testing.T) {
	// One Incubator artifact token with N +1/+1 counters → battlefield +1,
	// +N counters, +N P/T (Power()/Toughness() sum counters on the
	// noncreature token).
	mustPass(t, "incubate-2", mod("incubate", 2))
	mustPass(t, "incubate-5", mod("incubate", 5))
	mustSkip(t, "incubate-x", mod("incubate", "x"))
}

func TestWiden8_PayLife(t *testing.T) {
	mustPass(t, "pay-2-life", mod("pay_life", 2))
	mustPass(t, "pay-life-default", mod("pay_life")) // no arg → 1
	mustSkip(t, "pay-x-life", mod("pay_life", "x"))
}

func TestWiden8_ZeroDeltaFlags(t *testing.T) {
	mustPass(t, "regenerate", mod("regenerate", "simple"))
	mustPass(t, "goad", mod("goad"))
	mustPass(t, "take-initiative", mod("take_initiative"))
	mustPass(t, "no-life-gained", mod("no_life_gained"))
	mustPass(t, "choose-opponent", mod("choose_opponent"))
	mustPass(t, "plot", mod("plot"))
}

func TestWiden8_UnhandledModKindStaysOutOfScope(t *testing.T) {
	// An unhandled ModKind produces a zero snapshot delta via the engine's
	// parser_gap default; the interpreter must NOT assert zero for it
	// (that would mask the gap), so it stays out of phase-8 scope and
	// falls through to leafPhase3's out-of-scope catch-all.
	mustSkip(t, "emblem-typed", mod("emblem_typed", "creatures you control get +1/+1"))
	mustSkip(t, "detain", mod("detain", "chosen"))
}
