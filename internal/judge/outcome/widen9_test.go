package outcome

// Phase-9 unit suite — three whole effect TYPES newly modeled, each
// exercised end-to-end through RunEffect against the live engine, plus
// skip-path pins for the out-of-scope shapes.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestWiden9_SetLife(t *testing.T) {
	// Self: life moves from the scaffold's starting 20 to N.
	mustPass(t, "set-life-self-15", &gameast.SetLife{
		Amount: *gameast.NumInt(15), Target: gameast.Filter{Base: "self", Quantifier: "one"},
	})
	mustPass(t, "set-life-self-25", &gameast.SetLife{
		Amount: *gameast.NumInt(25), Target: gameast.Filter{Base: "self", Quantifier: "one"},
	})
	// Each-opponent.
	mustPass(t, "set-life-opp-10", &gameast.SetLife{
		Amount: *gameast.NumInt(10), Target: gameast.Filter{Base: "opponent", Quantifier: "one", Targeted: true},
	})
	// Policy-targeted "target player" → two-seat disjunction.
	mustPass(t, "set-life-target-player-1", &gameast.SetLife{
		Amount: *gameast.NumInt(1), Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true},
	})
	// Non-literal amount ("becomes your starting life total"): out of scope.
	mustSkip(t, "set-life-starting", &gameast.SetLife{
		Amount: gameast.NumberOrRef{IsStr: true, Str: "starting"},
		Target: gameast.Filter{Base: "self", Quantifier: "one"},
	})
}

func TestWiden9_CopyPermanentToken(t *testing.T) {
	// Token copy of a (4/4) creature: +1 battlefield, +4/+4.
	mustPass(t, "copy-token-that", &gameast.CopyPermanent{
		AsToken: true, Target: gameast.Filter{Base: "that", Quantifier: "one"},
	})
	mustPass(t, "copy-token-creature", &gameast.CopyPermanent{
		AsToken: true, Target: gameast.Filter{Base: "creature", Quantifier: "one"},
	})
	// Non-token clone (becomes a copy): layer-entangled, out of this band.
	mustSkip(t, "copy-nontoken", &gameast.CopyPermanent{
		AsToken: false, Target: gameast.Filter{Base: "that", Quantifier: "one"},
	})
	// Explicitly noncreature copy target: the engine copies a creature
	// regardless, a distinct question kept out of scope.
	mustSkip(t, "copy-token-artifact", &gameast.CopyPermanent{
		AsToken: true, Target: gameast.Filter{Base: "artifact", Quantifier: "one"},
	})
}

func TestWiden9_ExtraTurnCombat(t *testing.T) {
	mustPass(t, "extra-turn", &gameast.ExtraTurn{})
	mustPass(t, "extra-combat", &gameast.ExtraCombat{})
}
