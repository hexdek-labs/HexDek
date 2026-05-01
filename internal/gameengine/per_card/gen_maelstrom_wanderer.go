package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMaelstromWanderer wires Maelstrom Wanderer.
//
// Oracle text:
//
//   Creatures you control have haste.
//   Cascade, cascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it without paying its mana cost. Put the exiled cards on the bottom in a random order. Then do it again.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMaelstromWanderer(r *Registry) {
	r.OnETB("Maelstrom Wanderer", maelstromWandererStaticETB)
}

func maelstromWandererStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "maelstrom_wanderer_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
