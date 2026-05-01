package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMairsilThePretender wires Mairsil, the Pretender.
//
// Oracle text:
//
//   When Mairsil enters, you may exile an artifact or creature card from your hand or graveyard and put a cage counter on it.
//   Mairsil has all activated abilities of all cards you own in exile with cage counters on them. You may activate each of those abilities only once each turn.
//
// Auto-generated ETB handler.
func registerMairsilThePretender(r *Registry) {
	r.OnETB("Mairsil, the Pretender", mairsilThePretenderETB)
}

func mairsilThePretenderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mairsil_the_pretender_etb"
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
