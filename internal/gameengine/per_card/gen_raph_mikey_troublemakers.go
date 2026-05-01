package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaphMikeyTroublemakers wires Raph & Mikey, Troublemakers.
//
// Oracle text:
//
//   Trample, haste
//   Whenever Raph & Mikey attack, reveal cards from the top of your library until you reveal a creature card. Put that card onto the battlefield tapped and attacking and the rest on the bottom of your library in a random order.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRaphMikeyTroublemakers(r *Registry) {
	r.OnETB("Raph & Mikey, Troublemakers", raphMikeyTroublemakersStaticETB)
}

func raphMikeyTroublemakersStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "raph_mikey_troublemakers_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
