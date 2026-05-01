package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKalamaxTheStormsire wires Kalamax, the Stormsire.
//
// Oracle text:
//
//   Whenever you cast your first instant spell each turn, if Kalamax is tapped, copy that spell. You may choose new targets for the copy.
//   Whenever you copy an instant spell, put a +1/+1 counter on Kalamax.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKalamaxTheStormsire(r *Registry) {
	r.OnETB("Kalamax, the Stormsire", kalamaxTheStormsireStaticETB)
}

func kalamaxTheStormsireStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kalamax_the_stormsire_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
