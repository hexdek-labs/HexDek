package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophGreatestEarthbender wires Toph, Greatest Earthbender.
//
// Oracle text:
//
//   When Toph enters, earthbend X, where X is the amount of mana spent to cast her.
//   Land creatures you control have double strike.
//
// Auto-generated ETB handler.
func registerTophGreatestEarthbender(r *Registry) {
	r.OnETB("Toph, Greatest Earthbender", tophGreatestEarthbenderETB)
}

func tophGreatestEarthbenderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_greatest_earthbender_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
