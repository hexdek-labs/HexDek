package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRagavan wires up Ragavan, Nimble Pilferer.
//
// Oracle text:
//
//   Whenever Ragavan, Nimble Pilferer deals combat damage to a player,
//   create a Treasure token and exile the top card of that player's
//   library. Until end of turn, you may cast that card.
//
// Dash {1}{R} (You may cast this spell for its dash cost. If you do,
// it gains haste, and it's returned from the battlefield to its
// owner's hand at the beginning of the next end step.)
//
// cEDH staple — the combat damage trigger is the key mechanic. We
// register on the "combat_damage_dealt" trigger event. The dash
// alternative cost is handled by the generic AST resolver.
func registerRagavan(r *Registry) {
	r.OnTrigger("Ragavan, Nimble Pilferer", "combat_damage_player", ragavanCombatDamage)
}

func ragavanCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ragavan_combat_damage"
	if gs == nil || perm == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller || sourceName != "Ragavan, Nimble Pilferer" {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat == seat {
		// Ragavan hit its own controller (shouldn't happen, but guard).
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		// Default to first opponent.
		opps := gs.Opponents(seat)
		if len(opps) == 0 {
			return
		}
		defenderSeat = opps[0]
	}

	// Create a Treasure token for the controller.
	gameengine.CreateTreasureToken(gs, seat)

	// Exile top card of defender's library.
	defender := gs.Seats[defenderSeat]
	if len(defender.Library) == 0 {
		emit(gs, slug, "Ragavan, Nimble Pilferer", map[string]interface{}{
			"seat":          seat,
			"defender":      defenderSeat,
			"treasure":      true,
			"exiled":        false,
			"reason":        "defender_library_empty",
		})
		return
	}

	exiledCard := defender.Library[0]
	// Route through MoveCard so §614 replacements and triggers fire.
	gameengine.MoveCard(gs, exiledCard, defenderSeat, "library", "exile", "impulse-exile")

	gs.LogEvent(gameengine.Event{
		Kind:   "exile_from_library",
		Seat:   seat,
		Target: defenderSeat,
		Source: "Ragavan, Nimble Pilferer",
		Details: map[string]interface{}{
			"exiled_card": exiledCard.DisplayName(),
			"castable":    true,
			"until":       "end_of_turn",
		},
	})

	// Register zone-cast grant so the controller may cast the exiled card.
	gameengine.RegisterZoneCastGrant(gs, exiledCard, &gameengine.ZoneCastPermission{
		Zone:              "exile",
		Keyword:           "static_exile_cast",
		ManaCost:          -1, // use the card's normal mana cost
		RequireController: seat,
		SourceName:        "Ragavan, Nimble Pilferer",
	})

	emit(gs, slug, "Ragavan, Nimble Pilferer", map[string]interface{}{
		"seat":       seat,
		"defender":   defenderSeat,
		"treasure":   true,
		"exiled":     true,
		"exiled_card": exiledCard.DisplayName(),
	})
}
