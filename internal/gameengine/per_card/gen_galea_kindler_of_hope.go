package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGaleaKindlerOfHope wires Galea, Kindler of Hope.
//
// Oracle text:
//
//   Vigilance
//   You may look at the top card of your library any time.
//   You may cast Aura and Equipment spells from the top of your library. When you cast an Equipment spell this way, it gains "When this Equipment enters, attach it to target creature you control."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerGaleaKindlerOfHope(r *Registry) {
	r.OnETB("Galea, Kindler of Hope", galeaKindlerOfHopeStaticETB)
}

func galeaKindlerOfHopeStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "galea_kindler_of_hope_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
