package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGixYawgmothPraetor wires Gix, Yawgmoth Praetor.
//
// Oracle text:
//
//	Whenever a creature deals combat damage to one of your opponents,
//	its controller may pay 1 life. If they do, they draw a card.
//	{4}{B}{B}{B}, Discard X cards: Exile the top X cards of target
//	opponent's library. You may play lands and cast spells from among
//	cards exiled this way without paying their mana costs.
//
// Implementation:
//   - combat_damage_player: if damaged player is an opponent of Gix's
//     controller, the source creature's controller may pay 1 life to
//     draw a card. AI policy: take the deal whenever the controller
//     has > 5 life (a card is almost always worth 1 life).
//   - Activated discard-X / free-cast ability: emitPartial.
func registerGixYawgmothPraetor(r *Registry) {
	r.OnTrigger("Gix, Yawgmoth Praetor", "combat_damage_player", gixYawgmothCombatDamage)
}

func gixYawgmothCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "gix_yawgmoth_combat_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gixSeat := perm.Controller
	if gixSeat < 0 || gixSeat >= len(gs.Seats) {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	if defenderSeat == gixSeat {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat < 0 || sourceSeat >= len(gs.Seats) {
		return
	}
	src := gs.Seats[sourceSeat]
	if src == nil || src.Lost {
		return
	}
	if src.Life <= 5 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   gixSeat,
			"source": sourceSeat,
			"paid":   false,
		})
		return
	}
	gameengine.LoseLife(gs, sourceSeat, 1, perm.Card.DisplayName())
	drawOne(gs, sourceSeat, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   gixSeat,
		"source": sourceSeat,
		"paid":   true,
	})
	emitPartial(gs, "gix_yawgmoth_activated", perm.Card.DisplayName(),
		"discard_x_to_exile_and_free_cast_unimplemented")
}
