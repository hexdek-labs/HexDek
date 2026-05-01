package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShadowTheHedgehog wires Shadow the Hedgehog.
//
// Oracle text:
//
//   Haste
//   Whenever Shadow the Hedgehog or another creature you control with flash or haste dies, draw a card.
//   Chaos Control — Each spell you cast has split second if mana from an artifact was spent to cast it. (As long as it's on the stack, players can't cast spells or activate abilities that aren't mana abilities.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerShadowTheHedgehog(r *Registry) {
	r.OnETB("Shadow the Hedgehog", shadowTheHedgehogStaticETB)
}

func shadowTheHedgehogStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "shadow_the_hedgehog_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
