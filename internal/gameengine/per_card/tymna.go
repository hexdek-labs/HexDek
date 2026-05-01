package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTymna wires Tymna the Weaver.
//
// Oracle text:
//
//	At the beginning of your postcombat main phase, you may pay X life,
//	where X is the number of opponents that were dealt combat damage this
//	turn. If you do, draw X cards.
//	Partner
//
// The handler tracks per-turn opponent hits via perm.Flags["tymna_hit_<seat>"]
// (value = turn number it was hit). On the postcombat_main_controller trigger
// we count opponents hit this turn, pay X life, and draw X cards.
func registerTymna(r *Registry) {
	r.OnTrigger("Tymna the Weaver", "combat_damage_player", tymnaDamageObserver)
	r.OnTrigger("Tymna the Weaver", "postcombat_main_controller", tymnaPostcombatDraw)
}

func tymnaDamageObserver(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	if defenderSeat == perm.Controller {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["tymna_hit_"+strconv.Itoa(defenderSeat)] = gs.Turn
}

func tymnaPostcombatDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tymna_the_weaver_postcombat_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	x := 0
	if perm.Flags != nil {
		for _, opp := range gs.Opponents(perm.Controller) {
			if perm.Flags["tymna_hit_"+strconv.Itoa(opp)] == gs.Turn {
				x++
			}
		}
	}
	if x <= 0 {
		return
	}
	// "may pay X life" — decline if paying would kill us.
	if seat.Life <= x {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"x":       x,
			"paid":    false,
			"reason":  "lethal_life_cost",
			"life":    seat.Life,
		})
		return
	}

	seat.Life -= x
	gs.LogEvent(gameengine.Event{
		Kind:   "life_paid",
		Seat:   perm.Controller,
		Source: "Tymna the Weaver",
		Amount: x,
		Details: map[string]interface{}{
			"reason": "tymna_postcombat",
			"x":      x,
		},
	})
	for i := 0; i < x && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "draw")
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"x":         x,
		"paid":      true,
		"life_left": seat.Life,
	})
}
