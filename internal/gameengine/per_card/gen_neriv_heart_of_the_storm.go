package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNerivHeartOfTheStorm wires Neriv, Heart of the Storm.
//
// Oracle text:
//
//   Flying
//   If a creature you control that entered this turn would deal damage, it deals twice that much damage instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerNerivHeartOfTheStorm(r *Registry) {
	r.OnETB("Neriv, Heart of the Storm", nerivHeartOfTheStormStaticETB)
}

func nerivHeartOfTheStormStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "neriv_heart_of_the_storm_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
