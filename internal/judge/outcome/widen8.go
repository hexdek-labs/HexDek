package outcome

// widen8.go — OUTCOME interpreter phase 8 (r63b): a band of structured
// ModificationEffect kinds the parser DOES type with a concrete integer
// argument (or no argument at all), each resolving to a deterministic,
// snapshot-observable delta. Same independence contract: expected deltas
// from the AST + the harness's own BoardSpec, never from engine
// resolution.
//
// The two dominant out-of-scope ModKinds — parsed_effect_residual and
// untyped_effect — carry only a raw oracle string (the parser failed to
// structure them), so they remain genuinely unmodelable without
// re-implementing a text parser; they are NOT widened here. This phase
// takes the next band: the kinds whose Args ARE structured.
//
// New coverage classes:
//
//	MONSTROSITY  "monstrosity N" (CR §701.31): the source creature gains
//	    N +1/+1 counters and the monstrous designation. On the scaffold
//	    the source is a non-monstrous 4/4 creature, so the delta is
//	    exactly +N counters / +N power / +N toughness.
//	INVESTIGATE  "investigate N" (CR §701.36): N Clue artifact tokens
//	    enter under the controller. Clue tokens are 0/0 noncreature
//	    artifacts, so only the battlefield count moves (+N).
//	INCUBATE     "incubate N" (CR §701.46): one Incubator artifact token
//	    enters under the controller with N +1/+1 counters on it. The
//	    token is a noncreature artifact but Power()/Toughness() still sum
//	    its counters — battlefield +1, +N counters, +N power/toughness.
//	PAY-LIFE     "pay N life": the controller loses N life.
//	REGENERATE / GOAD  flag-only resolutions (regeneration shield, goad
//	    designation): zero snapshot-observable delta. A wrong
//	    implementation that draws/destroys/etc. instead is caught by the
//	    zero expectation.
//
// {X}/var amounts are deliberately OUT of scope for this band: the
// engine resolves these ModKinds with asInt / a float-or-int type
// assertion that fails on the "x"/"var" string and silently falls back
// to a literal default (1, or 2 for incubate). Modeling them from the
// AST's intended X would compare the interpreter's X against the
// engine's fallback — a real ambiguity, not a clean parity check — so
// litArg below accepts ONLY numeric literals and skips the rest.

import (
	"github.com/hexdek/hexdek/internal/gameast"
)

// litArg extracts a positive integer literal from a ModificationEffect
// arg slot. ok=false for string refs ("x"/"var"), missing args, or
// non-positive values — those are out of scope, never guessed.
func litArg(args []interface{}, idx int) (int, bool) {
	if idx < 0 || idx >= len(args) {
		return 0, false
	}
	switch v := args[idx].(type) {
	case int:
		if v > 0 {
			return v, true
		}
	case int64:
		if v > 0 {
			return int(v), true
		}
	case float64:
		if v > 0 {
			return int(v), true
		}
	}
	return 0, false
}

// leafPhase8 extends the leaf dispatch with the phase-8 structured
// ModKinds. Returns (handled, ok): handled=false → fall through to the
// earlier phases (notably leafPhase3, whose ModificationEffect catch-all
// would otherwise claim every ModKind as out of scope).
func leafPhase8(spec BoardSpec, eff gameast.Effect, d *Delta) (bool, bool) {
	e, ok := eff.(*gameast.ModificationEffect)
	if !ok {
		return false, false
	}
	switch e.ModKind {
	case "monstrosity":
		// CR §701.31 — only a non-monstrous creature can become monstrous.
		// The scaffold source is a fresh 4/4 creature with no monstrous
		// flag, so the engine puts exactly N +1/+1 counters on it.
		if !spec.SrcIsCreature {
			return true, false
		}
		n, ok := litArg(e.Args, 0)
		if !ok {
			return true, false
		}
		d.CountersByKind["+1/+1"] += n
		d.PowerSum += n
		d.ToughSum += n
		return true, true

	case "investigate":
		// CR §701.36 — N Clue tokens (0/0 noncreature artifacts) enter
		// under the controller.
		n, ok := litArg(e.Args, 0)
		if !ok {
			return true, false
		}
		d.BattlefieldBySeat[spec.Controller] += n
		return true, true

	case "incubate":
		// CR §701.46 — one Incubator artifact token enters under the
		// controller with N +1/+1 counters on it. The token's
		// Power()/Toughness() include its counters even though it is not
		// (yet) a creature, so the board P/T sums move by N.
		n, ok := litArg(e.Args, 0)
		if !ok {
			return true, false
		}
		d.BattlefieldBySeat[spec.Controller]++
		d.CountersByKind["+1/+1"] += n
		d.PowerSum += n
		d.ToughSum += n
		return true, true

	case "pay_life":
		// "pay N life" — the controller loses N life. The engine defaults
		// to 1 when no literal amount is present.
		n, ok := litArg(e.Args, 0)
		if !ok {
			if len(e.Args) != 0 {
				return true, false // a non-literal arg (X): out of scope
			}
			n = 1
		}
		d.LifeBySeat[spec.Controller] -= n
		return true, true

	case "regenerate", "goad", "take_initiative", "no_life_gained",
		"choose_opponent", "plot":
		// Flag/log-only resolutions with confirmed explicit engine
		// handlers, each changing no snapshot-observable count:
		//   regenerate     — regeneration shield flag (CR §701.15)
		//   goad           — goad designation on a target (CR §701.38)
		//   take_initiative— "has_initiative" seat designation (CR §722)
		//   no_life_gained — a "gain no life this turn" replacement marker
		//   choose_opponent— records a chosen opponent (no state change)
		//   plot           — "plotted" marker on the source (CR §702.165)
		// Zero delta expected; a resolver that draws/destroys/etc. instead
		// is caught against it. (Only ModKinds with an EXPLICIT handler are
		// listed — an unhandled ModKind also produces a zero snapshot delta
		// via the parser_gap default, so asserting zero there would mask a
		// genuine parser gap; those stay out of scope.)
		return true, true
	}
	return false, false
}
