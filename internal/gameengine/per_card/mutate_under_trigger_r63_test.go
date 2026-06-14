package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// CR §702.140: a merged (mutated) permanent has ALL the abilities of
// every card in the pile, regardless of which card is on top. Regression
// for the r63 mutate probe: the per_card trigger dispatcher matched
// handlers only by the surviving (top) card's name, so when a mutate card
// slid UNDER its target (onTop=false) the survivor bore the target's name
// and the mutate card's "Whenever this creature mutates, …" rider was
// unreachable — dead text. Dreamtail Heron ("Whenever this creature
// mutates, draw a card") mutating under a vanilla creature must still
// draw.

func mutate_dreamtailHeronCard(seat int) *gameengine.Card {
	return &gameengine.Card{
		Name:          "Dreamtail Heron",
		Owner:         seat,
		InstanceID:    "heron-inst-1",
		Types:         []string{"creature"},
		BasePower:     3,
		BaseToughness: 4,
		AST: &gameast.CardAST{
			Name: "Dreamtail Heron",
			Abilities: []gameast.Ability{
				&gameast.Keyword{Name: "flying", Raw: "Flying"},
				&gameast.Keyword{Name: "mutate", Raw: "Mutate {3}{U}"},
			},
		},
	}
}

func TestMutate_UnderTarget_FiresMutateTrigger(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top Card")

	// A vanilla non-Human target creature already on the battlefield.
	target := addPerm(gs, 0, "Pollywog Symbiote", "creature")
	target.Card.InstanceID = "target-inst-1"
	target.Card.BasePower, target.Card.BaseToughness = 1, 1

	// Dreamtail Heron in hand, to be cast for mutate and slid UNDER target.
	heron := mutate_dreamtailHeronCard(0)
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, heron)

	survivor, err := gameengine.CastWithMutate(gs, 0, heron, 0, target, false /*onTop*/)
	if err != nil {
		t.Fatalf("CastWithMutate(onTop=false) failed: %v", err)
	}
	// onTop=false → the TARGET survives as the single merged permanent.
	if survivor != target {
		t.Fatalf("expected target to be the surviving permanent on onTop=false")
	}
	if survivor.MergeKind == gameengine.MergeNone {
		t.Fatalf("survivor should be marked as a merged (mutate) permanent")
	}

	// The Heron's "Whenever this creature mutates, draw a card" rider must
	// have fired even though Heron is UNDER the target.
	if got := len(gs.Seats[0].Hand); got != 1 {
		t.Fatalf("Dreamtail Heron mutate-under: expected 1 card drawn, hand=%d", got)
	}
}

// Control: the mutate-card-ON-TOP path (survivor bears the mutate card's
// own name) must keep working — guards against the fix accidentally
// double-firing or skipping the common case.
func TestMutate_OnTopTarget_FiresMutateTriggerOnce(t *testing.T) {
	gs := newGame(t, 2)
	addLibrary(gs, 0, "Top Card A", "Top Card B")

	target := addPerm(gs, 0, "Pollywog Symbiote", "creature")
	target.Card.InstanceID = "target-inst-2"
	target.Card.BasePower, target.Card.BaseToughness = 1, 1

	heron := mutate_dreamtailHeronCard(0)
	heron.InstanceID = "heron-inst-2"
	gs.Seats[0].Hand = append(gs.Seats[0].Hand, heron)

	survivor, err := gameengine.CastWithMutate(gs, 0, heron, 0, target, true /*onTop*/)
	if err != nil {
		t.Fatalf("CastWithMutate(onTop=true) failed: %v", err)
	}
	if survivor.Card.DisplayName() != "Dreamtail Heron" {
		t.Fatalf("expected Heron on top, got %s", survivor.Card.DisplayName())
	}
	// Exactly one draw (once per mutate event), not zero or two.
	if got := len(gs.Seats[0].Hand); got != 1 {
		t.Fatalf("Dreamtail Heron mutate-on-top: expected exactly 1 draw, hand=%d", got)
	}
}
