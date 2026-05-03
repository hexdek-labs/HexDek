package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOmnathLocusOfCreation wires Omnath, Locus of Creation.
//
// Oracle text:
//
//   When Omnath enters, draw a card.
//   Landfall — Whenever a land you control enters, you gain 4 life if this is the first time this ability has resolved this turn. If it's the second time, add {R}{G}{W}{U}. If it's the third time, Omnath deals 4 damage to each opponent and each planeswalker you don't control.
//
// Auto-generated ETB handler.
func registerOmnathLocusOfCreation(r *Registry) {
	r.OnETB("Omnath, Locus of Creation", omnathLocusOfCreationETB)
}

func omnathLocusOfCreationETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "omnath_locus_of_creation_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
