package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInallaArchmageRitualist wires Inalla, Archmage Ritualist.
//
// Oracle text:
//
//   Eminence — Whenever another nontoken Wizard you control enters, if Inalla is in the command zone or on the battlefield, you may pay {1}. If you do, create a token that's a copy of that Wizard. The token gains haste. Exile it at the beginning of the next end step.
//   Tap five untapped Wizards you control: Target player loses 7 life.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerInallaArchmageRitualist(r *Registry) {
	r.OnETB("Inalla, Archmage Ritualist", inallaArchmageRitualistStaticETB)
}

func inallaArchmageRitualistStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "inalla_archmage_ritualist_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
