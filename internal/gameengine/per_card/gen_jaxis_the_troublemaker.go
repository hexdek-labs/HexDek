package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJaxisTheTroublemaker wires Jaxis, the Troublemaker.
//
// Oracle text:
//
//   {R}, {T}, Discard a card: Create a token that's a copy of another target creature you control. It gains haste and "When this token dies, draw a card." Sacrifice it at the beginning of the next end step. Activate only as a sorcery.
//   Blitz {1}{R} (If you cast this spell for its blitz cost, it gains haste and "When this creature dies, draw a card." Sacrifice it at the beginning of the next end step.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerJaxisTheTroublemaker(r *Registry) {
	r.OnETB("Jaxis, the Troublemaker", jaxisTheTroublemakerStaticETB)
}

func jaxisTheTroublemakerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "jaxis_the_troublemaker_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
