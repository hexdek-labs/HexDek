package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarumonixTheRatKing wires Karumonix, the Rat King.
//
// Oracle text:
//
//   Toxic 1 (Players dealt combat damage by this creature also get a poison counter.)
//   Other Rats you control have toxic 1.
//   When Karumonix enters, look at the top five cards of your library. You may reveal any number of Rat cards from among them and put the revealed cards into your hand. Put the rest on the bottom of your library in a random order.
//
// Auto-generated ETB handler.
func registerKarumonixTheRatKing(r *Registry) {
	r.OnETB("Karumonix, the Rat King", karumonixTheRatKingETB)
}

func karumonixTheRatKingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "karumonix_the_rat_king_etb"
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
