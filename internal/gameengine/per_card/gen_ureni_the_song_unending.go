package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUreniTheSongUnending wires Ureni, the Song Unending.
//
// Oracle text:
//
//   Flying, protection from white and from black
//   When Ureni enters, it deals X damage divided as you choose among any number of target creatures and/or planeswalkers your opponents control, where X is the number of lands you control.
//
// Auto-generated ETB handler.
func registerUreniTheSongUnending(r *Registry) {
	r.OnETB("Ureni, the Song Unending", ureniTheSongUnendingETB)
}

func ureniTheSongUnendingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ureni_the_song_unending_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
