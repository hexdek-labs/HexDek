package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrilTheMiststalker wires Uril, the Miststalker.
//
// Oracle text:
//
//   Hexproof (This creature can't be the target of spells or abilities your opponents control.)
//   Uril gets +2/+2 for each Aura attached to it.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerUrilTheMiststalker(r *Registry) {
	r.OnETB("Uril, the Miststalker", urilTheMiststalkerStaticETB)
}

func urilTheMiststalkerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "uril_the_miststalker_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
