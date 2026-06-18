package hat

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// prototype_policy_r63_test.go — CR §718 prototype AI policy. The default
// Hats prefer the full printed body when affordable and only take the cheaper
// prototype mode when mana-constrained (can't afford full, can afford
// prototype). card.CMC is the printed full cost (prototype is a non-destructive
// override), and `cost` is the prototype cost.
func TestGreedyHat_PrototypePolicy(t *testing.T) {
	h := &GreedyHat{}
	gs := newTestGame(t, 2)
	// card.CMC is the printed full cost (the policy's full-cost reference).
	card := &gameengine.Card{Name: "Skitterbeam Battalion", Types: []string{"artifact", "creature"}, CMC: 9}
	const protoCost = 5

	// Can afford full (9) → cast normally (decline prototype).
	gs.Seats[0].ManaPool = 9
	if got := h.ChooseOptionalCost(gs, 0, card, "prototype", protoCost, 1); got != 0 {
		t.Errorf("with full cost affordable, prototype should be declined; got %d", got)
	}

	// Can't afford full but can afford prototype → take prototype.
	gs.Seats[0].ManaPool = 6
	if got := h.ChooseOptionalCost(gs, 0, card, "prototype", protoCost, 1); got != 1 {
		t.Errorf("when mana-constrained but can afford prototype, take it; got %d", got)
	}

	// Can't afford either → decline (the cast will fail upstream).
	gs.Seats[0].ManaPool = 3
	if got := h.ChooseOptionalCost(gs, 0, card, "prototype", protoCost, 1); got != 0 {
		t.Errorf("can't afford prototype either → decline; got %d", got)
	}
}
