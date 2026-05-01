package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIndominusRexAlpha wires Indominus Rex, Alpha.
//
// Oracle text:
//
//   As Indominus Rex enters, discard any number of creature cards. It enters with a flying counter on it if a card discarded this way has flying. The same is true for first strike, double strike, deathtouch, hexproof, haste, indestructible, lifelink, menace, reach, trample, and vigilance.
//   When Indominus Rex enters, draw a card for each counter on it.
//
// Auto-generated ETB handler.
func registerIndominusRexAlpha(r *Registry) {
	r.OnETB("Indominus Rex, Alpha", indominusRexAlphaETB)
}

func indominusRexAlphaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "indominus_rex_alpha_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
