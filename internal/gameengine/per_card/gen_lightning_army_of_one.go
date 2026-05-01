package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLightningArmyOfOne wires Lightning, Army of One.
//
// Oracle text:
//
//   First strike, trample, lifelink
//   Stagger — Whenever Lightning deals combat damage to a player, until your next turn, if a source would deal damage to that player or a permanent that player controls, it deals double that damage instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerLightningArmyOfOne(r *Registry) {
	r.OnETB("Lightning, Army of One", lightningArmyOfOneStaticETB)
}

func lightningArmyOfOneStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lightning_army_of_one_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
