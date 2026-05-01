package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerToxrillTheCorrosive wires Toxrill, the Corrosive.
//
// Oracle text:
//
//   At the beginning of each end step, put a slime counter on each creature you don't control.
//   Creatures you don't control get -1/-1 for each slime counter on them.
//   Whenever a creature you don't control with a slime counter on it dies, create a 1/1 black Slug creature token.
//   {U}{B}, Sacrifice a Slug: Draw a card.
//
// Auto-generated trigger handler.
func registerToxrillTheCorrosive(r *Registry) {
	r.OnTrigger("Toxrill, the Corrosive", "end_step", toxrillTheCorrosiveTrigger1)
	r.OnTrigger("Toxrill, the Corrosive", "creature_dies", toxrillTheCorrosiveTrigger2)
}

func toxrillTheCorrosiveTrigger1(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toxrill_the_corrosive_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller { return }
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         perm.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func toxrillTheCorrosiveTrigger2(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toxrill_the_corrosive_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat == perm.Controller { return } // fires on opponent's creatures only
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         perm.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, perm.Controller, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
