package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSenTriplets wires Sen Triplets.
//
// Oracle text:
//
//   At the beginning of your upkeep, choose target opponent. This turn, that player can't cast spells or activate abilities and plays with their hand revealed. You may play lands and cast spells from that player's hand this turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSenTriplets(r *Registry) {
	r.OnETB("Sen Triplets", senTripletsStaticETB)
}

func senTripletsStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sen_triplets_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
