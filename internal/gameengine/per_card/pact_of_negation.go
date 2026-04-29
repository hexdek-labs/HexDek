package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPactOfNegation wires up Pact of Negation.
//
// Oracle text:
//
//	Counter target spell.
//	At the beginning of your next upkeep, pay {3}{U}{U}. If you
//	don't, you lose the game.
//
// Implementation:
//   - OnCast: register a delayed trigger for "your_next_upkeep"
//     that checks if the caster can pay 3UU. If they can, deduct
//     the mana. If they can't, they lose the game.
//   - The counter-spell effect itself is handled by the stock AST
//     CounterSpell resolver, so we only need the cast hook for the
//     delayed trigger.
func registerPactOfNegation(r *Registry) {
	r.OnCast("Pact of Negation", pactOfNegationOnCast)
}

func pactOfNegationOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "pact_of_negation_delayed"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: casterSeat,
		SourceCardName: "Pact of Negation",
		EffectFn: func(gs *gameengine.GameState) {
			if casterSeat < 0 || casterSeat >= len(gs.Seats) {
				return
			}
			seat := gs.Seats[casterSeat]
			if seat == nil || seat.Lost {
				return
			}
			// Try to pay {3}{U}{U} = 5 total, 2 of which must be U.
			// Use the typed mana pool if available.
			paid := false
			if seat.Mana != nil {
				if seat.Mana.U >= 2 && seat.Mana.Total() >= 5 {
					seat.Mana.U -= 2
					remaining := 3
					// Pay generic from colorless first, then any color.
					if seat.Mana.C >= remaining {
						seat.Mana.C -= remaining
						remaining = 0
					} else {
						remaining -= seat.Mana.C
						seat.Mana.C = 0
					}
					// Drain remaining from any available pool.
					for remaining > 0 {
						if seat.Mana.W > 0 {
							seat.Mana.W--
							remaining--
						} else if seat.Mana.U > 0 {
							seat.Mana.U--
							remaining--
						} else if seat.Mana.B > 0 {
							seat.Mana.B--
							remaining--
						} else if seat.Mana.R > 0 {
							seat.Mana.R--
							remaining--
						} else if seat.Mana.G > 0 {
							seat.Mana.G--
							remaining--
						} else if seat.Mana.Any > 0 {
							seat.Mana.Any--
							remaining--
						} else {
							break
						}
					}
					if remaining == 0 {
						paid = true
					}
				}
			} else if seat.ManaPool >= 5 {
				seat.ManaPool -= 5
				gameengine.SyncManaAfterSpend(seat)
				paid = true
			}

			if !paid {
				// Lose the game.
				seat.Lost = true
				seat.LossReason = "failed_to_pay_pact_of_negation"
				gs.LogEvent(gameengine.Event{
					Kind:   "lose_game",
					Seat:   casterSeat,
					Source: "Pact of Negation",
					Details: map[string]interface{}{
						"reason": "failed_to_pay_3UU",
						"rule":   "pact_trigger",
					},
				})
			} else {
				gs.LogEvent(gameengine.Event{
					Kind:   "mana_paid",
					Seat:   casterSeat,
					Source: "Pact of Negation",
					Details: map[string]interface{}{
						"cost": "3UU",
						"rule": "pact_trigger",
					},
				})
			}
		},
	})

	emit(gs, slug, "Pact of Negation", map[string]interface{}{
		"seat":    casterSeat,
		"delayed": "your_next_upkeep_pay_3UU_or_lose",
	})
}
