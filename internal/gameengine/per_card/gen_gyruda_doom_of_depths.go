package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGyrudaDoomOfDepths wires Gyruda, Doom of Depths.
//
// Oracle text:
//
//   Companion — Your starting deck contains only cards with even mana values. (If this card is your chosen companion, you may put it into your hand from outside the game for {3} as a sorcery.)
//   When Gyruda enters, each player mills four cards. Put a creature card with an even mana value from among the milled cards onto the battlefield under your control.
//
// Auto-generated ETB handler.
func registerGyrudaDoomOfDepths(r *Registry) {
	r.OnETB("Gyruda, Doom of Depths", gyrudaDoomOfDepthsETB)
}

func gyrudaDoomOfDepthsETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "gyruda_doom_of_depths_etb"
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
