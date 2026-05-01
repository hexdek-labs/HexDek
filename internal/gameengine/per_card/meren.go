package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMeren wires Meren of Clan Nel Toth.
//
// Oracle text:
//
//	Whenever another creature you control dies, you get an experience
//	counter.
//	At the beginning of your end step, choose target creature card in your
//	graveyard. If that card's mana value is less than or equal to the
//	number of experience counters you have, return it to the battlefield.
//	Otherwise, return it to your hand.
//
// Experience counters live at seat.Flags["experience_counters"], matching
// the engine's existing experience-counter wiring (resolve_helpers.go,
// scaling.go) so proliferate and ScalingAmount references see the same
// value.
func registerMeren(r *Registry) {
	r.OnTrigger("Meren of Clan Nel Toth", "creature_dies", merenDeathTrigger)
	r.OnTrigger("Meren of Clan Nel Toth", "end_step", merenEndStep)
}

func merenDeathTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	// "another creature" — exclude Meren herself.
	if dyingCard, _ := ctx["card"].(*gameengine.Card); dyingCard != nil && dyingCard == perm.Card {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["experience_counters"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "experience_counter",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"total": seat.Flags["experience_counters"],
		},
	})
}

func merenEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "meren_end_step_recursion"
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
	xp := 0
	if seat.Flags != nil {
		xp = seat.Flags["experience_counters"]
	}

	// Pick the highest-CMC creature that qualifies for battlefield return.
	var bfCard *gameengine.Card
	bfCMC := -1
	var handCard *gameengine.Card
	handCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc <= xp && cmc > bfCMC {
			bfCMC = cmc
			bfCard = c
		}
		if cmc > handCMC {
			handCMC = cmc
			handCard = c
		}
	}

	if bfCard != nil {
		gameengine.MoveCard(gs, bfCard, perm.Controller, "graveyard", "battlefield", "meren")
		enterBattlefieldWithETB(gs, perm.Controller, bfCard, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":        perm.Controller,
			"experience":  xp,
			"returned":    bfCard.DisplayName(),
			"cmc":         bfCMC,
			"destination": "battlefield",
		})
		return
	}
	if handCard != nil {
		gameengine.MoveCard(gs, handCard, perm.Controller, "graveyard", "hand", "meren")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":        perm.Controller,
			"experience":  xp,
			"returned":    handCard.DisplayName(),
			"cmc":         handCMC,
			"destination": "hand",
		})
		return
	}
	emitFail(gs, slug, perm.Card.DisplayName(), "no_creature_in_graveyard", map[string]interface{}{
		"seat":       perm.Controller,
		"experience": xp,
	})
}
