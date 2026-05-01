package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEdwardKenway wires Edward Kenway (Assassin's Creed crossover).
//
// Oracle text:
//
//	At the beginning of your end step, create a Treasure token for each
//	tapped Assassin, Pirate, and/or Vehicle you control.
//	Whenever a Vehicle you control deals combat damage to a player, look
//	at the top card of that player's library, then exile it face down.
//	You may play that card for as long as it remains exiled.
//
// Implementation:
//   - OnTrigger("end_step") — gates on active_seat == controller, walks
//     Edward's controller's battlefield, counts tapped permanents whose
//     subtypes include Assassin, Pirate, or Vehicle (each permanent
//     counts once even if it has multiple matching subtypes), and creates
//     N Treasure tokens.
//   - OnTrigger("combat_damage_player") — when a Vehicle controlled by
//     Edward's controller deals combat damage to a player, exile the top
//     card of the defender's library face-down and grant Edward's
//     controller permission to cast it from exile (ZoneCastPermission with
//     Zone:"exile", ManaCost: card's normal cost).
func registerEdwardKenway(r *Registry) {
	r.OnTrigger("Edward Kenway", "end_step", edwardEndStepTreasure)
	r.OnTrigger("Edward Kenway", "combat_damage_player", edwardVehicleDamage)
}

func edwardEndStepTreasure(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "edward_kenway_treasure_endstep"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.Tapped {
			continue
		}
		if cardHasType(p.Card, "assassin") || cardHasType(p.Card, "pirate") || cardHasType(p.Card, "vehicle") {
			count++
		}
	}
	if count == 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"treasures": 0,
		})
		return
	}

	for i := 0; i < count; i++ {
		token := &gameengine.Card{
			Name:  "Treasure Token",
			Owner: perm.Controller,
			Types: []string{"token", "artifact", "treasure"},
		}
		enterBattlefieldWithETB(gs, perm.Controller, token, false)
		gs.LogEvent(gameengine.Event{
			Kind:   "create_token",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"token":  "Treasure Token",
				"reason": "edward_kenway_endstep",
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"treasures": count,
	})
}

func edwardVehicleDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "edward_kenway_vehicle_exile_play"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName == "" {
		return
	}
	source := findEdwardSourcePerm(gs, sourceSeat, sourceName)
	if source == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "source_perm_not_found", map[string]interface{}{
			"seat":   perm.Controller,
			"source": sourceName,
		})
		return
	}
	if source.Card == nil || !cardHasType(source.Card, "vehicle") {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}
	if len(defender.Library) == 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "library_empty", map[string]interface{}{
			"seat":     perm.Controller,
			"defender": defenderSeat,
		})
		return
	}

	top := defender.Library[0]
	if top == nil {
		return
	}
	gameengine.MoveCard(gs, top, defenderSeat, "library", "exile", "edward_kenway_vehicle_exile")
	top.FaceDown = true
	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}
	gs.ZoneCastGrants[top] = &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "edward_kenway",
		ManaCost:          -1,
		RequireController: perm.Controller,
		SourceName:        perm.Card.DisplayName(),
	}
	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   defenderSeat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"card":      top.DisplayName(),
			"reason":    "edward_kenway_vehicle_damage",
			"face_down": true,
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
		"vehicle":       sourceName,
		"exiled_card":   top.DisplayName(),
	})
}

// findEdwardSourcePerm returns the first battlefield permanent on seat
// whose card's display name matches the given source name. Used to re-
// hydrate the Vehicle perm from a combat_damage_player ctx (which only
// carries source_card as a string).
func findEdwardSourcePerm(gs *gameengine.GameState, seatIdx int, name string) *gameengine.Permanent {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	s := gs.Seats[seatIdx]
	if s == nil {
		return nil
	}
	for _, p := range s.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() == name {
			return p
		}
	}
	return nil
}
