package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKardurDoomscourge wires Kardur, Doomscourge.
//
// Oracle text:
//
//   When Kardur enters, until your next turn, creatures your opponents control attack each combat if able and attack a player other than you if able.
//   Whenever an attacking creature dies, each opponent loses 1 life and you gain 1 life.
//
// Auto-generated ETB handler.
func registerKardurDoomscourge(r *Registry) {
	r.OnETB("Kardur, Doomscourge", kardurDoomscourgeETB)
}

func kardurDoomscourgeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kardur_doomscourge_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
