package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOzaiThePhoenixKing wires Ozai, the Phoenix King.
//
// Oracle text:
//
//   Trample, firebending 4, haste
//   If you would lose unspent mana, that mana becomes red instead.
//   Ozai has flying and indestructible as long as you have six or more unspent mana.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerOzaiThePhoenixKing(r *Registry) {
	r.OnETB("Ozai, the Phoenix King", ozaiThePhoenixKingStaticETB)
}

func ozaiThePhoenixKingStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ozai_the_phoenix_king_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
