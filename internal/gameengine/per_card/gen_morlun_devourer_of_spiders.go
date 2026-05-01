package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMorlunDevourerOfSpiders wires Morlun, Devourer of Spiders.
//
// Oracle text:
//
//   Lifelink
//   Morlun enters with X +1/+1 counters on him.
//   When Morlun enters, he deals X damage to target opponent.
//
// Auto-generated ETB handler.
func registerMorlunDevourerOfSpiders(r *Registry) {
	r.OnETB("Morlun, Devourer of Spiders", morlunDevourerOfSpidersETB)
}

func morlunDevourerOfSpidersETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "morlun_devourer_of_spiders_etb"
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
