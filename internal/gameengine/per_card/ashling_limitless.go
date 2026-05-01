package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAshlingTheLimitless wires Ashling the Limitless.
//
// Oracle text:
//
//	Whenever Ashling the Limitless deals damage to a player, exile that
//	many cards from the top of your library. Until end of turn, you may
//	play cards exiled this way.
//
// Implementation:
//   - Listens on "combat_damage_player". The printed trigger is "deals
//     damage to a player" (any damage), but the engine only fires the
//     combat damage event from per-card hook surface; non-combat damage
//     paths are not observed here. Since Ashling is a 4/4 elemental with
//     no built-in non-combat damage, this covers the realistic paths.
//   - On trigger: exile N cards from the top of the controller's library
//     where N = damage dealt, then register a ZoneCastPermission per
//     card so the AI/Hat treats them as castable from exile at normal
//     mana cost. Mirrors ob_nixilis_captive.go's impulse pattern.
//   - The "until end of turn" bound is not time-tracked (no end-step
//     cleanup); we emitPartial mirroring ob_nixilis_captive.
func registerAshlingTheLimitless(r *Registry) {
	r.OnTrigger("Ashling the Limitless", "combat_damage_player", ashlingTheLimitlessTrigger)
}

func ashlingTheLimitlessTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ashling_the_limitless_exile_play"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	amount, _ := ctx["amount"].(int)
	if sourceSeat != perm.Controller || amount <= 0 {
		return
	}
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	if gs.ZoneCastGrants == nil {
		gs.ZoneCastGrants = map[*gameengine.Card]*gameengine.ZoneCastPermission{}
	}

	exiled := []string{}
	for i := 0; i < amount && len(seat.Library) > 0; i++ {
		top := seat.Library[0]
		if top == nil {
			seat.Library = seat.Library[1:]
			continue
		}
		gameengine.MoveCard(gs, top, perm.Controller, "library", "exile", "ashling_limitless_exile")
		gs.ZoneCastGrants[top] = &gameengine.ZoneCastPermission{
			Zone:              "exile",
			Keyword:           "ashling_the_limitless_play",
			ManaCost:          -1,
			RequireController: perm.Controller,
			SourceName:        perm.Card.DisplayName(),
		}
		exiled = append(exiled, top.DisplayName())
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"damage":       amount,
		"exiled_count": len(exiled),
		"exiled":       exiled,
	})
	if len(exiled) > 0 {
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"play_window_until_end_of_turn_not_time_bounded")
	}
}
