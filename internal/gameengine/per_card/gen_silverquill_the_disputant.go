package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSilverquillTheDisputant wires Silverquill, the Disputant.
//
// Oracle text:
//
//   Flying, vigilance
//   Each instant and sorcery spell you cast has casualty 1. (As you cast that spell, you may sacrifice a creature with power 1 or greater. When you do, copy the spell and you may choose new targets for the copy.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSilverquillTheDisputant(r *Registry) {
	r.OnETB("Silverquill, the Disputant", silverquillTheDisputantStaticETB)
}

func silverquillTheDisputantStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "silverquill_the_disputant_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
