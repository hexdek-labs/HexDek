package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerElanorGardner wires Elanor Gardner (Muninn parser-gap #60,
// 12,603 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{3}{G}
//	Legendary Creature — Halfling Scout
//	When Elanor enters, create a Food token.
//	At the beginning of your end step, if you sacrificed a Food this
//	turn, you may search your library for a basic land card, put that
//	card onto the battlefield tapped, then shuffle.
//
// Implementation:
//   - ETB: drop a Food token via gameengine.CreateFoodToken.
//   - "If you sacrificed a Food this turn" tracking: listen on
//     OnTrigger("food_sacrificed") (the canonical event fired by
//     zone_change.go's sacrifice path). When fired, stamp
//     perm.Flags["elanor_food_sacced_turn"] = gs.Turn + 1 (turn+1 to
//     avoid zero-collision on turn 0). Only count sacs by the
//     controller (ctx["controller_seat"] == perm.Controller).
//   - End-step gate: active_seat == controller AND stamp == gs.Turn+1.
//     Search library for the first basic land card, MoveCard to
//     battlefield_tapped, shuffle.
//   - "you may" — Hat policy: always accept (free land drop is upside).
func registerElanorGardner(r *Registry) {
	r.OnETB("Elanor Gardner", elanorGardnerETB)
	r.OwnsETBTrigger("Elanor Gardner")
	r.OnTrigger("Elanor Gardner", "food_sacrificed", elanorGardnerFoodSacced)
	r.OnTrigger("Elanor Gardner", "end_step", elanorGardnerEndStep)
}

func elanorGardnerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "elanor_gardner_etb_food"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	gameengine.CreateFoodToken(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"token": "Food",
	})
}

func elanorGardnerFoodSacced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["elanor_food_sacced_turn"] = gs.Turn + 1
}

func elanorGardnerEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elanor_gardner_end_step_land"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil || perm.Flags["elanor_food_sacced_turn"] != gs.Turn+1 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"reason":    "no_food_sac_this_turn",
		})
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	var land *gameengine.Card
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		if cardHasType(c, "basic") && cardHasType(c, "land") {
			land = c
			break
		}
	}
	if land == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": true,
			"found":     "no_basic_land",
		})
		// Per CR §701.19c, shuffle still happens on whiff.
		shuffleLibraryPerCard(gs, perm.Controller)
		return
	}
	gameengine.MoveCard(gs, land, perm.Controller, "library", "battlefield_tapped", slug)
	shuffleLibraryPerCard(gs, perm.Controller)
	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"found":  []string{land.DisplayName()},
			"reason": slug,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"land":      land.DisplayName(),
	})
}
