package outcome

// Phase-6 unit suite — each new coverage class exercised end-to-end
// through RunEffect against the live engine, plus skip-path pins.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func TestWiden6_AbilityGrants(t *testing.T) {
	mustPass(t, "grant-flying-target", &gameast.GrantAbility{
		AbilityName: "flying",
		Target:      gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		Duration:    "until_end_of_turn",
	})
	mustPass(t, "grant-compound-self", &gameast.GrantAbility{
		AbilityName: "deathtouch and lifelink",
		Target:      gameast.Filter{Base: "self"},
		Duration:    "until_end_of_turn",
	})
	mustPass(t, "grant-each-own", &gameast.GrantAbility{
		AbilityName: "haste",
		Target:      gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true},
	})
	// Subtype-restricted grants ride the engine's fallback-to-source —
	// systemic engine-lane issue, refused here.
	mustSkip(t, "grant-subtype", &gameast.GrantAbility{
		AbilityName: "flying",
		Target:      gameast.Filter{Base: "creature", Quantifier: "one", CreatureTypes: []string{"wall"}},
	})
}

func TestWiden6_SelfRemovalAndAbsentBases(t *testing.T) {
	mustPass(t, "destroy-self", &gameast.Destroy{
		Target: gameast.Filter{Base: "self"},
	})
	mustPass(t, "exile-self", &gameast.Exile{
		Target: gameast.Filter{Base: "self"},
	})
	mustPass(t, "destroy-all-forests", &gameast.Destroy{
		Target: gameast.Filter{Base: "forests", Quantifier: "all"},
	})
	mustPass(t, "exile-all-walls", &gameast.Exile{
		Target: gameast.Filter{Base: "walls", Quantifier: "all"},
	})
}

func TestWiden6_TapAllSideDisjunction(t *testing.T) {
	// Plain "tap all creatures" (and the parser's rider-dropped
	// variants): the engine's actual must match one legal side-scope.
	mustPass(t, "tap-all-creatures", &gameast.TapEffect{
		Target: gameast.Filter{Base: "creature", Quantifier: "all"},
	})
}

func TestWiden6_DrainDiscardCounters(t *testing.T) {
	mustPass(t, "each-opponent-drain", &gameast.LoseLife{
		Amount: *gameast.NumInt(1),
		Target: gameast.Filter{Base: "opponent", Quantifier: "each"},
	})
	mustPass(t, "targeted-discard", &gameast.Discard{
		Count:  *gameast.NumInt(2),
		Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true},
	})
	mustPass(t, "double-zero-counters", &gameast.CounterMod{
		Op: "double", Count: *gameast.NumInt(1), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "target creature you control", Quantifier: "one", Targeted: true},
	})
	mustPass(t, "stifle-fizzles-on-spell", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "activated_ability", Quantifier: "one"},
	})
}

func TestWiden6_ReanimateAnyGraveyard(t *testing.T) {
	mustPass(t, "any-graveyard-return", &gameast.Reanimate{
		Query:    gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		FromZone: "any_graveyard", Destination: "battlefield", Controller: "you",
	})
}

func TestWiden6_UpToOneBounce(t *testing.T) {
	mustPass(t, "teferi-minus-three", &gameast.Bounce{
		Target: gameast.Filter{Base: "artifact", Quantifier: "up_to_n",
			Count: gameast.NumInt(1), Targeted: true},
		To: "owners_hand",
	})
}

func TestWiden6_ContextualUntapNoop(t *testing.T) {
	mustPass(t, "untap-it", &gameast.UntapEffect{
		Target: gameast.Filter{Base: "that_thing"},
	})
	mustPass(t, "untap-enchanted", &gameast.UntapEffect{
		Target: gameast.Filter{Base: "enchanted_creature"},
	})
}

// The suspend-style guard: counters placed after the source left the
// battlefield are invisible to the snapshot census — refuse to guess.
func TestWiden6_SuspendCountersOutOfScope(t *testing.T) {
	mustSkip(t, "arc-blade", &gameast.Sequence{Items: []gameast.Effect{
		&gameast.Exile{Target: gameast.Filter{Base: "self"}},
		&gameast.CounterMod{Op: "put", Count: *gameast.NumInt(3), CounterKind: "time",
			Target: gameast.Filter{Base: "self"}},
	}})
}
