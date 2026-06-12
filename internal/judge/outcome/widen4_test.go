package outcome

// Phase-4 unit suite: each new kind exercised THROUGH the engine via
// RunEffect (a finding means expected≠actual; nil finding + ran=true
// means the engine matched the independent expectation), plus
// skip-path pins for the shapes the interpreter must refuse to guess.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func mustPass(t *testing.T, name string, eff gameast.Effect) {
	t.Helper()
	fd, ran := RunEffect(name, "", eff)
	if !ran {
		t.Fatalf("%s: expected in scope, got skipped", name)
	}
	if fd != nil {
		t.Fatalf("%s: diverged: expected %s, actual %s", name, fd.Expected, fd.Actual)
	}
}

func mustSkip(t *testing.T, name string, eff gameast.Effect) {
	t.Helper()
	if _, ran := RunEffect(name, "", eff); ran {
		t.Fatalf("%s: expected out-of-scope skip, but it ran", name)
	}
}

func TestWiden4_TutorShapes(t *testing.T) {
	mustPass(t, "tutor-basic-to-hand", &gameast.Tutor{
		Query: gameast.Filter{Base: "basic land_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "hand", ShuffleAfter: true,
	})
	// The underscore emission — pinned against the engine-side matcher
	// fix (tutor_underscore_base_r63_test.go owns the engine half).
	mustPass(t, "tutor-underscore-base", &gameast.Tutor{
		Query: gameast.Filter{Base: "basic_land_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "battlefield_tapped", ShuffleAfter: true,
	})
	mustPass(t, "tutor-to-battlefield", &gameast.Tutor{
		Query: gameast.Filter{Base: "land", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "battlefield",
	})
	// Optional tutor: declined ∪ taken disjunction.
	mustPass(t, "tutor-optional", &gameast.Tutor{
		Query: gameast.Filter{Base: "basic land_card", Quantifier: "one"},
		Count: *gameast.NumInt(1), Destination: "hand", Optional: true,
	})
	// Subtype-restricted query: may not match the scaffold library.
	mustSkip(t, "tutor-subtype", &gameast.Tutor{
		Query: gameast.Filter{Base: "basic land_card", CreatureTypes: []string{"forest"}},
		Count: *gameast.NumInt(1), Destination: "hand",
	})
	// More copies than stocked candidates.
	mustSkip(t, "tutor-overcount", &gameast.Tutor{
		Query: gameast.Filter{Base: "basic land_card"},
		Count: *gameast.NumInt(99), Destination: "hand",
	})
}

func TestWiden4_BounceShapes(t *testing.T) {
	// Harmful single-target bounce hits the opponent (policy pin).
	mustPass(t, "bounce-opp-creature", &gameast.Bounce{
		Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true},
		To:     "owners_hand",
	})
	// Self-bounce returns the source to the controller's hand.
	mustPass(t, "bounce-self", &gameast.Bounce{
		Target: gameast.Filter{Base: "self"},
	})
	// "permanent" is set-valued (creature pick shifts P/T sums): skip.
	mustSkip(t, "bounce-permanent", &gameast.Bounce{
		Target: gameast.Filter{Base: "permanent", Quantifier: "one", Targeted: true},
	})
}

func TestWiden4_MassRemoval(t *testing.T) {
	mustPass(t, "wrath", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "all"},
	})
	mustPass(t, "exile-all-artifacts", &gameast.Exile{
		Target: gameast.Filter{Base: "artifact", Quantifier: "all"},
	})
	mustSkip(t, "wrath-subtype", &gameast.Destroy{
		Target: gameast.Filter{Base: "creature", Quantifier: "all", CreatureTypes: []string{"forest"}},
	})
}

func TestWiden4_SacrificeShapes(t *testing.T) {
	mustPass(t, "sac-own-creature", &gameast.Sacrifice{
		Query: gameast.Filter{Base: "creature"},
	})
	mustPass(t, "each-player-sacs", &gameast.Sacrifice{
		Query: gameast.Filter{Base: "creature"}, Actor: "each_player",
	})
	// Wider bases are set-valued across tapped/creature picks: skip.
	mustSkip(t, "sac-permanent", &gameast.Sacrifice{
		Query: gameast.Filter{Base: "permanent"},
	})
}

func TestWiden4_CounterSpell(t *testing.T) {
	// The harness stocks one live opponent spell for counter shapes;
	// resolution marks it Countered → live stack -1.
	mustPass(t, "plain-counter", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell", Quantifier: "one"},
	})
	// "unless pays {1}": the scaffold opponent has an empty pool, so
	// the counter still lands.
	mustPass(t, "unless-counter", &gameast.CounterSpell{
		Target: gameast.Filter{Base: "spell", Quantifier: "one"},
		Unless: &gameast.Cost{Mana: &gameast.ManaCost{Symbols: []gameast.ManaSymbol{{Generic: 1}}}},
	})
}

func TestWiden4_ZeroDeltaAndSurveil(t *testing.T) {
	mustPass(t, "scry", &gameast.Scry{Count: *gameast.NumInt(2)})
	mustPass(t, "reveal", &gameast.Reveal{Count: *gameast.NumInt(3)})
	mustPass(t, "shuffle", &gameast.Shuffle{})
	mustPass(t, "surveil", &gameast.Surveil{Count: *gameast.NumInt(2)})
}

func TestWiden4_DamageShapes(t *testing.T) {
	mustPass(t, "bolt-any-target", &gameast.Damage{
		Amount: *gameast.NumInt(3),
		Target: gameast.Filter{Base: "any_target", Targeted: true},
	})
	mustPass(t, "pyroclasm-sublethal", &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "creature", Quantifier: "each"},
	})
	mustPass(t, "earthquake-players", &gameast.Damage{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "player", Quantifier: "each"},
	})
	// Lethal mass damage: out of the sublethal envelope.
	mustSkip(t, "lethal-sweep", &gameast.Damage{
		Amount: *gameast.NumInt(5),
		Target: gameast.Filter{Base: "creature", Quantifier: "each"},
	})
}

func TestWiden4_FightAndManaAndGraveyard(t *testing.T) {
	mustPass(t, "fight", &gameast.Fight{
		A: gameast.Filter{Base: "creature", Targeted: true},
		B: gameast.Filter{Base: "creature", Targeted: true},
	})
	mustPass(t, "bite", &gameast.Fight{
		A: gameast.Filter{Base: "creature", Targeted: true},
		B: gameast.Filter{Base: "creature", Targeted: true}, OneSided: true,
	})
	mustPass(t, "add-mana", &gameast.AddMana{
		Pool: []gameast.ManaSymbol{{Color: []string{"G"}}, {Color: []string{"G"}}},
	})
	mustPass(t, "add-any-color", &gameast.AddMana{AnyColorCount: 1})
	mustPass(t, "reanimate", &gameast.Reanimate{
		Query: gameast.Filter{Base: "creature_card", Targeted: true},
	})
	mustPass(t, "recurse", &gameast.Recurse{
		Query: gameast.Filter{Base: "creature_card", Targeted: true},
	})
	// Reanimation riders move more state than the plain return: skip.
	mustSkip(t, "reanimate-with-mods", &gameast.Reanimate{
		Query:             gameast.Filter{Base: "creature_card"},
		WithModifications: []string{"+1/+1_counter"},
	})
}

func TestWiden4_FanOutCounters(t *testing.T) {
	mustPass(t, "counters-each-own", &gameast.CounterMod{
		Op: "put", Count: *gameast.NumInt(1), CounterKind: "+1/+1",
		Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true},
	})
	// Board evolved earlier in the sequence (tokens created first):
	// the fan-out count is no longer the static scaffold — skip.
	mustSkip(t, "tokens-then-counters", &gameast.Sequence{Items: []gameast.Effect{
		&gameast.CreateToken{Count: *gameast.NumInt(4), PT: &[2]int{1, 1}, Types: []string{"creature"}},
		&gameast.CounterMod{Op: "put", Count: *gameast.NumInt(1), CounterKind: "+1/+1",
			Target: gameast.Filter{Base: "creature", Quantifier: "each", YouControl: true}},
	}})
}

func TestWiden4_LoseLifeTargetPlayer(t *testing.T) {
	mustPass(t, "target-player-loses", &gameast.LoseLife{
		Amount: *gameast.NumInt(2),
		Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true},
	})
}
