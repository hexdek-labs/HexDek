package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKraum wires up Kraum, Ludevic's Opus.
//
// Oracle text:
//
//	Flying, haste
//	Whenever an opponent casts their second spell each turn, you draw
//	a card.
//	Partner
//
// Implementation: listen on "spell_cast" and check the per-seat
// SpellsCastThisTurn counter. When an opponent's count hits exactly 2,
// Kraum's controller draws a card.
//
// The counter is already incremented by IncrementCastCount (cast_counts.go)
// BEFORE fireCastTriggers fires the "spell_cast" event, so we can read
// Seats[caster].SpellsCastThisTurn directly.
func registerKraum(r *Registry) {
	r.OnTrigger("Kraum, Ludevic's Opus", "spell_cast", kraumTrigger)
}

func kraumTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	controller := perm.Controller

	casterSeat, ok := ctx["caster_seat"].(int)
	if !ok || casterSeat == controller {
		return // only opponents trigger Kraum
	}

	// Check this opponent's spell count this turn.
	if casterSeat < 0 || casterSeat >= len(gs.Seats) || gs.Seats[casterSeat] == nil {
		return
	}
	count := gs.Seats[casterSeat].SpellsCastThisTurn
	if count != 2 {
		return // only the second spell triggers the draw
	}

	// Draw a card for Kraum's controller.
	drawOne(gs, controller, perm.Card.DisplayName())
	gs.LogEvent(gameengine.Event{
		Kind:   "cast_trigger_observer",
		Seat:   controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"slug":        "kraum",
			"caster_seat": casterSeat,
			"spell_count": count,
			"effect":      fmt.Sprintf("draw_card_opponent_2nd_spell_seat_%d", casterSeat),
			"rule":        "603.2",
		},
	})
}
