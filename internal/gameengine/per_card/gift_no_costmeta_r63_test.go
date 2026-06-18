package per_card

import (
	"testing"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// gift_no_costmeta_r63_test.go — r63 mechanic-probe (CR §702.192, Gift).
//
// A gift spell cast WITHOUT promising a gift carries no gift metadata
// (nil CostMeta is the normal not-promised case, and the only case that
// ever happened before the cast-side wiring existed). The resolve
// handlers must still run the spell's BASE effect in no-gift mode.
// Previously giftPromised returned (false,false) for nil CostMeta and
// every handler abstained — so the base effect (the 2 damage, the
// destroy) was lost and the card did literally nothing.

func TestGift_NilCostMeta_RunsBaseEffect(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[1].Life = 20
	target := addPerm(gs, 1, "Big Bear", "creature")
	target.Card.BasePower, target.Card.BaseToughness = 5, 5

	// StackItem with NO CostMeta — the not-promised cast.
	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Blooming Blast", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	if target.MarkedDamage != 2 {
		t.Errorf("base effect must run in no-gift mode: want 2 damage to creature, got %d", target.MarkedDamage)
	}
	if gs.Seats[1].Life != 20 {
		t.Errorf("no gift-gated bonus when not promised: opp life should stay 20, got %d", gs.Seats[1].Life)
	}
}

// Nocturnal Hunger's inverted rider: NOT promised → destroy + lose 2 life.
// Confirms the no-gift (nil CostMeta) branch reaches the penalty too.
func TestGift_NilCostMeta_InvertedRiderApplies(t *testing.T) {
	gs := newGame(t, 2)
	gs.Seats[0].Life = 20
	victim := addPerm(gs, 1, "Doomed Creature", "creature")
	victim.Card.BasePower, victim.Card.BaseToughness = 3, 3

	item := &gameengine.StackItem{
		Controller: 0,
		Card:       &gameengine.Card{Name: "Nocturnal Hunger", Owner: 0, Types: []string{"instant"}},
	}
	gameengine.InvokeResolveHook(gs, item)

	// Creature destroyed (base effect) AND caster loses 2 life (the
	// "if the gift WASN'T promised" penalty).
	if gs.Seats[0].Life != 18 {
		t.Errorf("not-promised inverted rider: caster should lose 2 life (20→18), got %d", gs.Seats[0].Life)
	}
}
