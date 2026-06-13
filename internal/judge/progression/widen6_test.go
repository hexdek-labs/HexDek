package progression

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
)

// widen6_test.go — pins the r63d phase-3d scope gate (turned_face_up).

func tfuDraw() gameast.Effect {
	return &gameast.Draw{Count: gameast.NumberOrRef{IsInt: true, Int: 1}, Target: gameast.Filter{Base: "you"}}
}

func TestTurnedFaceUp_InScopeSelf(t *testing.T) {
	for _, raw := range []string{
		"when this creature is turned face up, draw a card",
		"when ~ is turned face up, draw a card",
	} {
		tr := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: "turned_face_up"},
			Effect:  tfuDraw(),
			Raw:     raw,
		}
		if !InScopeTurnedFaceUpTrigger(tr) {
			t.Fatalf("self turned-face-up trigger must be in scope: %q", raw)
		}
	}
}

func TestTurnedFaceUp_GatesObserverAndRiders(t *testing.T) {
	for _, raw := range []string{
		"whenever a permanent is turned face up, you may draw a card", // observer (actor dropped)
		"when this creature is turned face up, if you control a forest, draw a card", // if-rider
	} {
		tr := &gameast.Triggered{
			Trigger: gameast.Trigger{Event: "turned_face_up"},
			Effect:  tfuDraw(),
			Raw:     raw,
		}
		if InScopeTurnedFaceUpTrigger(tr) {
			t.Fatalf("out-of-scope turned-face-up wording must fail closed: %q", raw)
		}
	}
}

func TestTurnedFaceUp_GatesWrongEvent(t *testing.T) {
	tr := &gameast.Triggered{
		Trigger: gameast.Trigger{Event: "etb"},
		Effect:  tfuDraw(),
		Raw:     "when this creature enters, draw a card",
	}
	if InScopeTurnedFaceUpTrigger(tr) {
		t.Fatal("non turned_face_up event must be out of scope")
	}
}
