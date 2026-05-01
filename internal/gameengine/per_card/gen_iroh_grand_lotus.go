package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIrohGrandLotus wires Iroh, Grand Lotus.
//
// Oracle text:
//
//   Firebending 2
//   During your turn, each non-Lesson instant and sorcery card in your graveyard has flashback. The flashback cost is equal to that card's mana cost. (You may cast a card from your graveyard for its flashback cost. Then exile it.)
//   During your turn, each Lesson card in your graveyard has flashback {1}.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerIrohGrandLotus(r *Registry) {
	r.OnETB("Iroh, Grand Lotus", irohGrandLotusStaticETB)
}

func irohGrandLotusStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "iroh_grand_lotus_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
