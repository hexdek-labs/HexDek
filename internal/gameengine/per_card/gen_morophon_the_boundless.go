package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMorophonTheBoundless wires Morophon, the Boundless.
//
// Oracle text:
//
//   Changeling (This card is every creature type.)
//   As Morophon enters, choose a creature type.
//   Spells of the chosen type you cast cost {W}{U}{B}{R}{G} less to cast. This effect reduces only the amount of colored mana you pay.
//   Other creatures you control of the chosen type get +1/+1.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMorophonTheBoundless(r *Registry) {
	r.OnETB("Morophon, the Boundless", morophonTheBoundlessStaticETB)
}

func morophonTheBoundlessStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "morophon_the_boundless_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
