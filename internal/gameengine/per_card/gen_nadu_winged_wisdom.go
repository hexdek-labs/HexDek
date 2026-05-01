package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNaduWingedWisdom wires Nadu, Winged Wisdom.
//
// Oracle text:
//
//   Flying
//   Creatures you control have "Whenever this creature becomes the target of a spell or ability, reveal the top card of your library. If it's a land card, put it onto the battlefield. Otherwise, put it into your hand. This ability triggers only twice each turn."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerNaduWingedWisdom(r *Registry) {
	r.OnETB("Nadu, Winged Wisdom", naduWingedWisdomStaticETB)
}

func naduWingedWisdomStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "nadu_winged_wisdom_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
