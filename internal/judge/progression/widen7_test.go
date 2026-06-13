package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// widen7_test.go — pins the r63 saturation-sweep scope gates (draw_card).

func dcGain() gameast.Effect {
	return &gameast.GainLife{Amount: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}}
}

func TestDrawCard_InScopeYouDraw(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "draw_card"},
		Effect:  dcGain(),
		Raw:     "whenever you draw a card, you gain 1 life",
	}
	if !InScopeDrawCardTrigger(tr) {
		t.Fatal("'whenever you draw a card' must be in scope")
	}
}

func TestDrawCard_GatesRidersAndOtherEvents(t *testing.T) {
	for _, c := range []struct {
		ev, raw string
	}{
		{"draw_card", "whenever you draw a card, except the first one each turn, you gain 1 life"},
		{"draw_card", "whenever you draw a card, if you have no cards in hand, you gain 1 life"},
		{"you_whenever", "whenever you draw your second card each turn, you gain 1 life"},
		{"etb", "when this enters, you gain 1 life"},
	} {
		tr := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: c.ev},
			Effect:  dcGain(),
			Raw:     c.raw,
		}
		if InScopeDrawCardTrigger(tr) {
			t.Fatalf("must be out of scope: %q", c.raw)
		}
	}
}

func TestSacrifice_InScopeFilters(t *testing.T) {
	cases := map[string]string{
		"whenever you sacrifice a creature, put a +1/+1 counter on this creature":     "creature",
		"whenever you sacrifice an artifact, this creature deals 2 damage to a player": "artifact",
		"whenever you sacrifice another creature, put a +1/+1 counter on this creature": "creature",
		"whenever you sacrifice a permanent, you gain 1 life":                          "any",
	}
	for raw, want := range cases {
		tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: "sacrifice_filtered"}, Effect: dcGain(), Raw: raw}
		sf, ok := InScopeSacrificeTrigger(tr)
		if !ok {
			t.Fatalf("must be in scope: %q", raw)
		}
		if sf.typ != want {
			t.Errorf("filter for %q = %q, want %q", raw, sf.typ, want)
		}
	}
}

func TestSacrifice_GatesBoardScalingAndCascade(t *testing.T) {
	for _, raw := range []string{
		"whenever you sacrifice another permanent, creatures you control get +1/+0 until end of turn",
		"whenever you sacrifice a creature, target opponent sacrifices a creature of their choice",
	} {
		tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: "sacrifice_filtered"}, Effect: dcGain(), Raw: raw}
		if _, ok := InScopeSacrificeTrigger(tr); ok {
			t.Fatalf("board-scaling / cascade sacrifice effect must be out of scope: %q", raw)
		}
	}
}

func TestDiscard_InScopeYouDiscard(t *testing.T) {
	tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: "discard_filtered"}, Effect: dcGain(),
		Raw: "whenever you discard a card, you gain 1 life"}
	if !InScopeDiscardTrigger(tr) {
		t.Fatal("'whenever you discard a card' must be in scope")
	}
	// Type-filtered / wrong-event forms out of scope.
	for _, c := range []struct{ ev, raw string }{
		{"discard_filtered", "whenever you discard a creature card, you gain 1 life"},
		{"etb", "when this enters, you gain 1 life"},
	} {
		tr := &gameast.Triggered{Trigger: gameast.Trigger{Event: c.ev}, Effect: dcGain(), Raw: c.raw}
		if InScopeDiscardTrigger(tr) {
			t.Fatalf("must be out of scope: %q", c.raw)
		}
	}
}
