package outcome

// Unit self-tests: the interpreter + harness agree with the engine on
// known-good effects, and a seeded wrong-amount bug IS caught — the
// outcome dimension has teeth, not just coverage.

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

func intRef(n int) gameast.NumberOrRef { return gameast.NumberOrRef{IsInt: true, Int: n} }

func TestOutcome_KnownGoodEffectsPass(t *testing.T) {
	cases := []struct {
		name string
		eff  gameast.Effect
	}{
		{"draw 2", &gameast.Draw{Count: intRef(2), Target: gameast.Filter{Base: "self"}}},
		{"gain 3 life", &gameast.GainLife{Amount: intRef(3), Target: gameast.Filter{Base: "you"}}},
		{"lose 2 life self", &gameast.LoseLife{Amount: intRef(2), Target: gameast.Filter{Base: "self"}}},
		{"each opponent loses 2", &gameast.LoseLife{Amount: intRef(2), Target: gameast.Filter{Base: "each_opponent"}}},
		{"deal 3 to creature", &gameast.Damage{Amount: intRef(3), Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true}}},
		{"deal 2 to player", &gameast.Damage{Amount: intRef(2), Target: gameast.Filter{Base: "player", Quantifier: "one", Targeted: true}}},
		{"create 2 tokens", &gameast.CreateToken{Count: intRef(2), PT: &[2]int{1, 1}, Types: []string{"creature", "soldier"}}},
		{"destroy creature", &gameast.Destroy{Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true}}},
		{"exile artifact", &gameast.Exile{Target: gameast.Filter{Base: "artifact", Quantifier: "one", Targeted: true}}},
		{"put 2 +1/+1 on self", &gameast.CounterMod{Op: "put", Count: intRef(2), CounterKind: "+1/+1", Target: gameast.Filter{Base: "self"}}},
	}
	for _, c := range cases {
		finding, ran := RunEffect("unit:"+c.name, c.name, c.eff)
		if !ran {
			t.Errorf("%s: unexpectedly out of scope", c.name)
			continue
		}
		if finding != nil {
			t.Errorf("%s: engine diverged from interpreter:\n  expected %s\n  actual   %s",
				c.name, finding.Expected, finding.Actual)
		}
	}
}

// Teeth check: corrupt the expectation (simulating an engine that
// resolves the wrong amount) — the comparator must flag it.
func TestOutcome_SeededWrongAmountCaught(t *testing.T) {
	spec := DefaultSpec()
	eff := &gameast.GainLife{Amount: intRef(3), Target: gameast.Filter{Base: "you"}}
	expected, ok := Expect(spec, eff)
	if !ok {
		t.Fatal("gain 3 life must be in scope")
	}
	// The engine "resolves" gain 5 instead of 3.
	gs, src := BuildBoard(spec, "seeded-bug")
	before := snap(gs)
	wrong := &gameast.GainLife{Amount: intRef(5), Target: gameast.Filter{Base: "you"}}
	resolveThroughEngine(gs, src, wrong)
	actual := diff(before, snap(gs))
	if actual.Equal(expected) {
		t.Fatal("comparator failed to catch a wrong-amount resolution")
	}
}

func TestOutcome_OutOfScopeIsSkippedNotGuessed(t *testing.T) {
	outOfScope := []gameast.Effect{
		&gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "devotion"}, Target: gameast.Filter{Base: "creature"}},
		&gameast.Damage{Amount: intRef(3), Target: gameast.Filter{Base: "creature", Quantifier: "each"}},
		&gameast.Exile{Target: gameast.Filter{Base: "creature"}, Until: "leaves_battlefield"},
		&gameast.CreateToken{Count: intRef(1), IsCopyOf: &gameast.Filter{Base: "creature"}},
		&gameast.Destroy{Target: gameast.Filter{Base: "creature", CreatureTypes: []string{"dragon"}}},
	}
	for i, eff := range outOfScope {
		if _, ok := Expect(DefaultSpec(), eff); ok {
			t.Errorf("case %d (%s): must be out of phase-1 scope", i, eff.Kind())
		}
	}
}


// ---- r63 part 2: widened-kind unit coverage -------------------------------

func TestOutcome_WidenedKindsPass(t *testing.T) {
	cases := []struct {
		name string
		eff  gameast.Effect
	}{
		{"deal X (pinned 3) to creature", &gameast.Damage{Amount: gameast.NumberOrRef{IsStr: true, Str: "x"}, Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true}}},
		{"mill 3 self", &gameast.Mill{Count: intRef(3), Target: gameast.Filter{Base: "self"}}},
		{"opponent discards 2", &gameast.Discard{Count: intRef(2), Target: gameast.Filter{Base: "opponent"}}},
		{"tap target creature", &gameast.TapEffect{Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true}}},
		{"untap target land", &gameast.UntapEffect{Target: gameast.Filter{Base: "land", Quantifier: "one", Targeted: true}}},
		{"pump +2/+2 until EOT", &gameast.Buff{Power: 2, Toughness: 2, Duration: "until_end_of_turn", Target: gameast.Filter{Base: "creature", Quantifier: "one", Targeted: true}}},
	}
	for _, c := range cases {
		finding, ran := RunEffect("unit:"+c.name, c.name, c.eff)
		if !ran {
			t.Errorf("%s: unexpectedly out of scope", c.name)
			continue
		}
		if finding != nil {
			t.Errorf("%s: engine diverged:\n  expected %s\n  actual   %s", c.name, finding.Expected, finding.Actual)
		}
	}
}

// Modal/optional/conditional produce expectation SETS; the engine's
// pick must match SOME member.
func TestOutcome_DisjunctiveShapes(t *testing.T) {
	choice := &gameast.Choice{
		Pick: intRef(1),
		Options: []gameast.Effect{
			&gameast.Draw{Count: intRef(1), Target: gameast.Filter{Base: "self"}},
			&gameast.GainLife{Amount: intRef(4), Target: gameast.Filter{Base: "you"}},
		},
	}
	if set, ok := ExpectSet(DefaultSpec(), choice); !ok || len(set) != 2 {
		t.Fatalf("choose-one must produce a 2-element expectation set, ok=%v n=%d", ok, len(set))
	}
	if finding, ran := RunEffect("unit:choose-one", "choose one", choice); !ran {
		t.Fatal("choose-one unexpectedly out of scope")
	} else if finding != nil {
		t.Errorf("engine's modal pick matched no expected mode:\n  %s vs %s", finding.Actual, finding.Expected)
	}

	opt := &gameast.Optional_{Body: &gameast.GainLife{Amount: intRef(2), Target: gameast.Filter{Base: "you"}}}
	if finding, ran := RunEffect("unit:optional", "you may gain 2", opt); !ran {
		t.Fatal("optional unexpectedly out of scope")
	} else if finding != nil {
		t.Errorf("optional resolution matched neither taken nor declined: %s", finding.Actual)
	}
}

// PART 1 class regression: the up_to_n harmful-removal class (the
// dimension's first catch, fixed via PickTargetHarmful) reports ZERO
// divergence — destroy up to 3 must remove exactly the opponent's
// creature and never the controller's own.
func TestOutcome_UpToNRemovalClass_ZeroDivergence(t *testing.T) {
	upTo3 := &gameast.Destroy{Target: gameast.Filter{
		Base: "creature", Quantifier: "up_to_n",
		Count: &gameast.NumberOrRef{IsInt: true, Int: 3}, Targeted: true,
	}}
	// up_to_n removal is set-valued in principle but the harness board
	// has exactly one opponent creature: expect exactly its removal.
	finding, ran := RunEffect("unit:up-to-3-destroy", "destroy up to three target creatures", upTo3)
	if !ran {
		t.Skip("up_to_n removal out of interpreter scope on this board")
	}
	if finding != nil {
		t.Fatalf("up_to_n removal class diverged (the #1052 catch regressed):\n  expected %s\n  actual %s",
			finding.Expected, finding.Actual)
	}
}
