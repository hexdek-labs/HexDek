package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTannukMemorialEnsign wires Tannuk, Memorial Ensign.
//
// Oracle text:
//
//   Landfall — Whenever a land you control enters, Tannuk deals 1 damage to each opponent. If this is the second time this ability has resolved this turn, draw a card.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTannukMemorialEnsign(r *Registry) {
	r.OnETB("Tannuk, Memorial Ensign", tannukMemorialEnsignStaticETB)
}

func tannukMemorialEnsignStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tannuk_memorial_ensign_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
