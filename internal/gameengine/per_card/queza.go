package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQueza wires Queza, Augur of Agonies.
//
// Oracle text:
//
//	Whenever you draw a card, target opponent loses 1 life and you gain
//	1 life.
//
// Trigger on the canonical "card_drawn" event. Picks the highest-life
// living opponent and drains them for 1 (controller gains 1).
func registerQueza(r *Registry) {
	r.OnTrigger("Queza, Augur of Agonies", "card_drawn", quezaDrawTrigger)
}

func quezaDrawTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "queza_drain_on_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	if drawerSeat != perm.Controller {
		return
	}
	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	bestOpp := -1
	bestLife := -1 << 30
	for _, opp := range opps {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			bestOpp = opp
		}
	}
	if bestOpp < 0 {
		return
	}
	gs.Seats[bestOpp].Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   bestOpp,
		Source: perm.Card.DisplayName(),
		Amount: 1,
	})
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": bestOpp,
		"drain":  1,
	})
}
