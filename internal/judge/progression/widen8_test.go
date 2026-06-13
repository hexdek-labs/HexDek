package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// widen8_test.go — pins the r63 ally-death scope gates.

func adGain() gameast.Effect {
	return &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}}
}

func TestAllyDies_InScopeForms(t *testing.T) {
	cases := map[string]struct {
		ev            string
		anyController bool
		another       bool
	}{
		"whenever a creature you control dies, you gain 1 life":       {"creature_dies", false, false},
		"whenever another creature you control dies, you gain 1 life":  {"another_creature_dies", false, true},
		"whenever a creature dies, you gain 1 life":                    {"creature_dies", true, false},
	}
	for raw, want := range cases {
		tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: want.ev}, Effect: adGain(), Raw: raw}
		sc, ok := InScopeAllyDiesTrigger(tr)
		if !ok {
			t.Fatalf("must be in scope: %q", raw)
		}
		if sc.anyController != want.anyController || sc.excludesSelf != want.another {
			t.Errorf("%q scope = %+v, want any=%v another=%v", raw, sc, want.anyController, want.another)
		}
	}
}

func TestAllyDies_GatesConditionalAndScaling(t *testing.T) {
	for _, raw := range []string{
		"whenever a creature you control with a +1/+1 counter on it dies, draw a card",
		"whenever a creature you control with power or toughness 1 or less dies, you gain 1 life",
		"whenever a creature or planeswalker you control dies, you gain 1 life",
		"whenever a creature you control dies, creatures you control get +1/+1 until end of turn",
	} {
		tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: "creature_dies"}, Effect: adGain(), Raw: raw}
		if _, ok := InScopeAllyDiesTrigger(tr); ok {
			t.Fatalf("conditional/scaling ally-dies wording must be out of scope: %q", raw)
		}
	}
}
