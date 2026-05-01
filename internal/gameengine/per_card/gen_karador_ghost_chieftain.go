package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKaradorGhostChieftain wires Karador, Ghost Chieftain.
//
// Oracle text:
//
//   This spell costs {1} less to cast for each creature card in your graveyard.
//   Once during each of your turns, you may cast a creature spell from your graveyard.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKaradorGhostChieftain(r *Registry) {
	r.OnETB("Karador, Ghost Chieftain", karadorGhostChieftainStaticETB)
}

func karadorGhostChieftainStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "karador_ghost_chieftain_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
