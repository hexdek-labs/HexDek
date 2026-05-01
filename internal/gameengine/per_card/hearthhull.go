package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHearthhull wires Hearthhull, the Worldseed.
//
// Oracle text:
//
//	Station (Tap another creature you control: Put charge counters equal
//	to its power on this Spacecraft. Station only as a sorcery. It's an
//	artifact creature at 8+.)
//	2+ | {1}, {T}, Sacrifice a land: Draw two cards. You may play an
//	additional land this turn.
//	8+ | Flying, vigilance, haste
//	Whenever you sacrifice a land, each opponent loses 2 life.
//
// Implementation:
//
//   - ETB: emitPartial flagging that the Spacecraft Station mechanic and
//     charge-counter gating (2+/8+ thresholds) are unmodeled.
//   - OnActivated: resolves the {1},{T},Sac-a-land ability — costs are
//     paid by the engine, we apply the effect (draw 2 + extra land
//     drop). We do NOT gate on the 2+ station threshold; emitPartial
//     records that nuance.
//   - OnTrigger("permanent_sacrificed"): when a land controlled by
//     Hearthhull's controller is sacrificed, each opponent loses 2 life.
func registerHearthhull(r *Registry) {
	r.OnETB("Hearthhull, the Worldseed", hearthhullETB)
	r.OnActivated("Hearthhull, the Worldseed", hearthhullActivate)
	r.OnTrigger("Hearthhull, the Worldseed", "permanent_sacrificed", hearthhullLandSacDrain)
}

func hearthhullETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hearthhull_station_unmodeled"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"station_charge_counters_and_8plus_creature_threshold_unimplemented")
}

func hearthhullActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "hearthhull_draw_two_extra_land"
	if gs == nil || src == nil {
		return
	}
	seat := gs.Seats[src.Controller]
	if seat == nil || seat.Lost {
		return
	}

	drawn := 0
	for i := 0; i < 2; i++ {
		if len(seat.Library) == 0 {
			break
		}
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, src.Controller, "library", "hand", "hearthhull_draw")
		drawn++
	}

	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["extra_land_drops"]++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_land_drop",
		Seat:   src.Controller,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":   slug,
			"reason": "hearthhull_activated",
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":  src.Controller,
		"drawn": drawn,
	})
	emitPartial(gs, slug, src.Card.DisplayName(),
		"station_2plus_threshold_gating_unimplemented")
}

func hearthhullLandSacDrain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hearthhull_land_sac_drain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	controllerSeat, _ := ctx["controller_seat"].(int)
	if card == nil || !cardHasType(card, "land") {
		return
	}
	if controllerSeat != perm.Controller {
		return
	}

	drained := 0
	for i, s := range gs.Seats {
		if s == nil || i == perm.Controller || s.Lost {
			continue
		}
		s.Life -= 2
		drained++
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   perm.Controller,
			Target: i,
			Source: perm.Card.DisplayName(),
			Amount: -2,
			Details: map[string]interface{}{
				"slug":   slug,
				"reason": "hearthhull_land_sacrificed",
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"sacrificed_land":   card.DisplayName(),
		"opponents_drained": drained,
	})
}
