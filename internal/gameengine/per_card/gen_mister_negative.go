package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMisterNegative wires Mister Negative.
//
// Oracle text:
//
//   Vigilance, lifelink
//   Darkforce Inversion — When Mister Negative enters, you may exchange life totals with target opponent. If you lost life this way, draw that many cards.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMisterNegative(r *Registry) {
	r.OnETB("Mister Negative", misterNegativeStaticETB)
}

func misterNegativeStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mister_negative_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
