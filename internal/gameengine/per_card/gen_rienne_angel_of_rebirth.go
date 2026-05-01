package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRienneAngelOfRebirth wires Rienne, Angel of Rebirth.
//
// Oracle text:
//
//   Flying
//   Other multicolored creatures you control get +1/+0.
//   Whenever another multicolored creature you control dies, return it to its owner's hand at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRienneAngelOfRebirth(r *Registry) {
	r.OnETB("Rienne, Angel of Rebirth", rienneAngelOfRebirthStaticETB)
}

func rienneAngelOfRebirthStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rienne_angel_of_rebirth_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
