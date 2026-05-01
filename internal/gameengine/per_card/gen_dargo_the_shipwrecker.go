package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDargoTheShipwrecker wires Dargo, the Shipwrecker.
//
// Oracle text:
//
//   As an additional cost to cast this spell, you may sacrifice any number of artifacts and/or creatures. This spell costs {2} less to cast for each permanent sacrificed this way and {2} less to cast for each other artifact or creature you've sacrificed this turn.
//   Trample
//   Partner (You can have two commanders if both have partner.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerDargoTheShipwrecker(r *Registry) {
	r.OnETB("Dargo, the Shipwrecker", dargoTheShipwreckerStaticETB)
}

func dargoTheShipwreckerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dargo_the_shipwrecker_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
