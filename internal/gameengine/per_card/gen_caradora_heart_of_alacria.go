package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaradoraHeartOfAlacria wires Caradora, Heart of Alacria.
//
// Oracle text:
//
//   When Caradora enters, you may search your library for a Mount or Vehicle card, reveal it, put it into your hand, then shuffle.
//   If one or more +1/+1 counters would be put on a creature or Vehicle you control, that many plus one +1/+1 counters are put on it instead.
//
// Auto-generated ETB handler.
func registerCaradoraHeartOfAlacria(r *Registry) {
	r.OnETB("Caradora, Heart of Alacria", caradoraHeartOfAlacriaETB)
}

func caradoraHeartOfAlacriaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "caradora_heart_of_alacria_etb"
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
