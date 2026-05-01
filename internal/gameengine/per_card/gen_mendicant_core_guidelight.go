package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMendicantCoreGuidelight wires Mendicant Core, Guidelight.
//
// Oracle text:
//
//   Mendicant Core's power is equal to the number of artifacts you control.
//   Start your engines! (If you have no speed, it starts at 1. It increases once on each of your turns when an opponent loses life. Max speed is 4.)
//   Max speed — Whenever you cast an artifact spell, you may pay {1}. If you do, copy it. (The copy becomes a token.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMendicantCoreGuidelight(r *Registry) {
	r.OnETB("Mendicant Core, Guidelight", mendicantCoreGuidelightStaticETB)
}

func mendicantCoreGuidelightStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mendicant_core_guidelight_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
