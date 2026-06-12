package outcome

// Phase-5 unit suite — every new coverage class exercised end-to-end
// through RunEffect against the live engine (a nil finding means the
// independent expectation matched actual engine behavior), plus
// skip-path pins for refuse-to-guess shapes.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestWiden5_ZeroDeltaResolutions(t *testing.T) {
	mustPass(t, "prevent-shield", &gameast.Prevent{Amount: *gameast.NumInt(2)})
	mustPass(t, "untap-self-noop", &gameast.UntapEffect{
		Target: gameast.Filter{Base: "self"},
	})
	mustPass(t, "regenerate", &gameast.ModificationEffect{
		ModKind: "regenerate_typed",
		Args:    []interface{}{},
	})
	mustPass(t, "restriction", &gameast.ModificationEffect{
		ModKind: "restriction",
		Args:    []interface{}{"target creature can't block this turn"},
	})
	mustPass(t, "cast-trigger-tail", &gameast.ModificationEffect{
		ModKind: "cast_trigger_tail",
		Args:    []interface{}{},
	})
}

func TestWiden5_TargetedDrawDisjunction(t *testing.T) {
	// WHICH player draws is policy; the amount is rules. The engine's
	// current pick (either seat) must match one disjunct.
	mustPass(t, "ancestral-recall", &gameast.Draw{
		Count:  *gameast.NumInt(3),
		Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true},
	})
}

func TestWiden5_DistributeCounters(t *testing.T) {
	// The split is policy, the TOTAL is rules. Pre-fix the engine
	// dropped op "distribute" entirely (31-card silent no-op cluster —
	// Ajani Mentor of Heroes' +1, Armament Corps, Dueling Coach).
	mustPass(t, "distribute-two", &gameast.CounterMod{
		Op: "distribute", Count: *gameast.NumInt(2), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "one or two target creatures you control", Quantifier: "any", Targeted: true},
	})
	mustPass(t, "distribute-three", &gameast.CounterMod{
		Op: "distribute", Count: *gameast.NumInt(3), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "up to three target creatures", Quantifier: "any", Targeted: true},
	})
}

func TestWiden5_RestrictedRemoval(t *testing.T) {
	// Zero-candidate adjective: no tapped/attacking creature exists on
	// the scaffold — the removal must fizzle.
	mustPass(t, "destroy-tapped-creature", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, Extra: []string{"tapped"}},
	})
	mustPass(t, "destroy-attacking-creature", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, Extra: []string{"attacking"}},
	})
	// Positive color on the colorless scaffold: never matches.
	mustPass(t, "destroy-green-creature", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, ColorFilter: []string{"G"}},
	})
	// ColorExclude pass-through (Doom Blade): colorless is never
	// excluded — behaves as plain removal.
	mustPass(t, "doom-blade", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, ColorExclude: []string{"B"}},
	})
	// Unknown adjective: not provably zero-candidate — skip.
	mustSkip(t, "destroy-enchanted-creature", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, Extra: []string{"enchanted"}},
	})
}

func TestWiden5_BounceWidened(t *testing.T) {
	mustPass(t, "bounce-own-creature", &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, YouControl: true},
		To:     "owners_hand",
	})
	mustPass(t, "bounce-tapped-creature-fizzle", &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true, Extra: []string{"tapped"}},
	})
}

func TestWiden5_TutorFailToFindAndPools(t *testing.T) {
	// Nothing of these types is stocked: guaranteed legal fail-to-find.
	mustPass(t, "tutor-artifact-miss", &gameast.Tutor{
		Query: gameast.Filter{Base: "artifact_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "hand", ShuffleAfter: true,
	})
	mustPass(t, "tutor-instant-miss", &gameast.Tutor{
		Query: gameast.Filter{Base: "instant_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "battlefield",
	})
	// Positive-color query against colorless fillers: miss.
	mustPass(t, "tutor-green-miss", &gameast.Tutor{
		Query: gameast.Filter{Base: "creature_card", Quantifier: "one", ColorFilter: []string{"G"}},
		Count: *gameast.NumInt(1), Destination: "hand",
	})
	// Creature pool: filler creatures are stocked.
	mustPass(t, "tutor-creature-hit", &gameast.Tutor{
		Query: gameast.Filter{Base: "creature_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "hand",
	})
	// Mana-value filters: CMC-0 fillers may match — refuse to guess.
	mustSkip(t, "tutor-mv-filter", func() gameast.Effect {
		mv := 3
		return &gameast.Tutor{
			Query: gameast.Filter{Base: "creature_card", ManaValueOp: "<=", ManaValue: &mv},
			Count: *gameast.NumInt(1), Destination: "hand",
		}
	}())
}

func TestWiden5_CopySpell(t *testing.T) {
	// The staged opponent stack spell is copied: live stack +1.
	mustPass(t, "fork", &gameast.CopySpell{
		Target: gameast.Filter{Base: "spell", Quantifier: "one", Targeted: true},
	})
}

func TestWiden5_AnyTargetAltEmission(t *testing.T) {
	// The parser's second any-target shape: {base:"target",
	// quantifier:"any"} (Acorn Catapult class).
	mustPass(t, "acorn-catapult", &gameast.Damage{
		Amount: *gameast.NumInt(1),
		Target: gameast.Filter{Base: "target", Quantifier: "any"},
	})
}
