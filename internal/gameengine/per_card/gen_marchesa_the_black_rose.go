package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMarchesaTheBlackRose wires Marchesa, the Black Rose.
//
// Oracle text:
//
//   Dethrone (Whenever this creature attacks the player with the most life or tied for most life, put a +1/+1 counter on it.)
//   Other creatures you control have dethrone.
//   Whenever a creature you control with a +1/+1 counter on it dies, return that card to the battlefield under your control at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMarchesaTheBlackRose(r *Registry) {
	r.OnETB("Marchesa, the Black Rose", marchesaTheBlackRoseStaticETB)
}

func marchesaTheBlackRoseStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "marchesa_the_black_rose_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
