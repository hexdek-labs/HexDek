package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheReaperKingNoMore wires The Reaper, King No More.
//
// Oracle text:
//
//   When The Reaper enters, put a -1/-1 counter on each of up to two target creatures.
//   Whenever a creature an opponent controls with a -1/-1 counter on it dies, you may put that card onto the battlefield under your control. Do this only once each turn.
//
// Auto-generated ETB handler.
func registerTheReaperKingNoMore(r *Registry) {
	r.OnETB("The Reaper, King No More", theReaperKingNoMoreETB)
}

func theReaperKingNoMoreETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_reaper_king_no_more_etb"
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
