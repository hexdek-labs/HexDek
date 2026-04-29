package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChainsOfMephistopheles wires Chains of Mephistopheles.
//
// Oracle text:
//
//	If a player would draw a card except the first one they draw in
//	each of their draw steps, that player discards a card instead.
//	If the player discards a card this way, they draw a card.
//	If the player doesn't discard a card this way, they mill a card.
//
// This is a replacement effect on draws. Implementation:
//   - Register a "draw_replacement" trigger that fires on every draw event.
//   - Check if the draw is the first draw-step draw (exempt).
//   - Otherwise: if the player has cards in hand, discard one then draw one.
//     If the player has no cards in hand, mill one.
//
// CR §614.1d: Chains replaces the draw event itself.
func registerChainsOfMephistopheles(r *Registry) {
	r.OnTrigger("Chains of Mephistopheles", "player_would_draw", chainsDrawReplacement)
}

func chainsDrawReplacement(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "chains_of_mephistopheles"
	if gs == nil || perm == nil {
		return
	}

	drawSeat, _ := ctx["draw_seat"].(int)
	if drawSeat < 0 || drawSeat >= len(gs.Seats) {
		return
	}

	// Check: is this the first draw in the draw step?
	// The draw step sets a flag "_suppress_first_draw_trigger_seat" in gs.Flags.
	// If the current draw is the draw-step draw, it's exempt from Chains.
	if gs.Flags != nil && gs.Flags["_chains_first_draw_exempt"] > 0 {
		// First draw in draw step is exempt; consume the flag.
		delete(gs.Flags, "_chains_first_draw_exempt")
		return
	}

	seat := gs.Seats[drawSeat]
	if seat == nil {
		return
	}

	// Set ctx flag to indicate the draw was replaced.
	if ctx != nil {
		ctx["draw_replaced"] = true
	}

	if len(seat.Hand) > 0 {
		// Player has cards — discard one, then draw one.
		// Hat picks which card to discard; fallback: last card.
		var toDiscard *gameengine.Card
		if seat.Hat != nil {
			choices := seat.Hat.ChooseDiscard(gs, drawSeat, seat.Hand, 1)
			if len(choices) > 0 {
				toDiscard = choices[0]
			}
		}
		if toDiscard == nil && len(seat.Hand) > 0 {
			toDiscard = seat.Hand[len(seat.Hand)-1]
		}

		if toDiscard != nil {
			gameengine.DiscardCard(gs, toDiscard, drawSeat)
			gs.LogEvent(gameengine.Event{
				Kind:   "discard",
				Seat:   drawSeat,
				Source: toDiscard.DisplayName(),
				Details: map[string]interface{}{
					"reason": "chains_of_mephistopheles",
					"rule":   "614.1d",
				},
			})

			// Now draw one card (the replacement draw).
			if len(seat.Library) > 0 {
				c := seat.Library[0]
				gameengine.MoveCard(gs, c, drawSeat, "library", "hand", "draw")
				gs.LogEvent(gameengine.Event{
					Kind:   "draw",
					Seat:   drawSeat,
					Source: c.DisplayName(),
					Amount: 1,
					Details: map[string]interface{}{
						"reason": "chains_replacement_draw",
						"rule":   "614.1d",
					},
				})
			}
		}
	} else {
		// Player has no cards in hand — mill one.
		if len(seat.Library) > 0 {
			c := seat.Library[0]
			gameengine.MoveCard(gs, c, drawSeat, "library", "graveyard", "mill")
			gs.LogEvent(gameengine.Event{
				Kind:   "mill",
				Seat:   drawSeat,
				Source: c.DisplayName(),
				Amount: 1,
				Details: map[string]interface{}{
					"reason": "chains_no_discard_mill",
					"rule":   "614.1d",
				},
			})
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"draw_seat":  drawSeat,
		"had_cards":  len(seat.Hand) > 0,
	})
}
