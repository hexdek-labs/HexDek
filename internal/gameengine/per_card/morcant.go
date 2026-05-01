package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMorcant wires High Perfect Morcant.
//
// Oracle text:
//
//	When High Perfect Morcant enters, draw cards equal to the number of
//	opponents.
//	Whenever you discard a card, each opponent loses 1 life.
//
// Implementation:
//   - OnETB: draw one card per living opponent.
//   - OnTrigger("card_discarded"): when the discarder is Morcant's
//     controller, drain each opponent for 1.
func registerMorcant(r *Registry) {
	r.OnETB("High Perfect Morcant", morcantETB)
	r.OnTrigger("High Perfect Morcant", "card_discarded", morcantDiscardTrigger)
}

func morcantETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "morcant_etb_draw"
	if gs == nil || perm == nil {
		return
	}
	opps := gs.Opponents(perm.Controller)
	drawn := 0
	for range opps {
		if drawOne(gs, perm.Controller, perm.Card.DisplayName()) != nil {
			drawn++
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"opponents": len(opps),
		"drawn":     drawn,
	})
}

func morcantDiscardTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "morcant_discard_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat != perm.Controller {
		return
	}
	drained := 0
	for _, oppIdx := range gs.Opponents(perm.Controller) {
		opp := gs.Seats[oppIdx]
		if opp == nil {
			continue
		}
		opp.Life -= 1
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   oppIdx,
			Target: oppIdx,
			Source: perm.Card.DisplayName(),
			Amount: -1,
			Details: map[string]interface{}{
				"reason": "morcant_discard",
			},
		})
		drained++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"drained":  drained,
		"per_opp":  1,
	})
}
