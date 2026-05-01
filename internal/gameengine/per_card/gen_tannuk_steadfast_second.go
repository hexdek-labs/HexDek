package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTannukSteadfastSecond wires Tannuk, Steadfast Second.
//
// Oracle text:
//
//   Other creatures you control have haste.
//   Artifact cards and red creature cards in your hand have warp {2}{R}. (You may cast a card from your hand for its warp cost. Exile that permanent at the beginning of the next end step, then you may cast it from exile on a later turn.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTannukSteadfastSecond(r *Registry) {
	r.OnETB("Tannuk, Steadfast Second", tannukSteadfastSecondStaticETB)
}

func tannukSteadfastSecondStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tannuk_steadfast_second_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
