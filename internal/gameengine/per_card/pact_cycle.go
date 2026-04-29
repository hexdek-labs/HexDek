package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPactOfTheTitan wires up Pact of the Titan.
//
// Oracle text:
//
//	Create a 4/4 red Giant creature token.
//	At the beginning of your next upkeep, pay {4}{R}. If you
//	don't, you lose the game.
func registerPactOfTheTitan(r *Registry) {
	r.OnCast("Pact of the Titan", pactOfTheTitanOnCast)
}

func pactOfTheTitanOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "pact_of_the_titan_delayed"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Create 4/4 red Giant creature token.
	token := &gameengine.Card{
		Name:          "Giant Token",
		Owner:         casterSeat,
		Types:         []string{"token", "creature"},
		Colors:        []string{"R"},
		BasePower:     4,
		BaseToughness: 4,
	}
	enterBattlefieldWithETB(gs, casterSeat, token, false)

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: casterSeat,
		SourceCardName: "Pact of the Titan",
		EffectFn:       pactUpkeepPayOrLose(casterSeat, "Pact of the Titan", 5, 1, "R"),
	})

	emit(gs, slug, "Pact of the Titan", map[string]interface{}{
		"seat":    casterSeat,
		"token":   "4/4 Giant",
		"delayed": "your_next_upkeep_pay_4R_or_lose",
	})
}

// registerSlaughterPact wires up Slaughter Pact.
//
// Oracle text:
//
//	Destroy target nonblack creature.
//	At the beginning of your next upkeep, pay {2}{B}. If you
//	don't, you lose the game.
func registerSlaughterPact(r *Registry) {
	r.OnCast("Slaughter Pact", slaughterPactOnCast)
}

func slaughterPactOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "slaughter_pact_delayed"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Destroy target nonblack creature (pick opponent's best creature).
	for _, oppIdx := range gs.Opponents(casterSeat) {
		opp := gs.Seats[oppIdx]
		if opp == nil {
			continue
		}
		for _, p := range opp.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			// Check nonblack.
			isBlack := false
			if p.Card != nil {
				for _, c := range p.Card.Colors {
					if c == "B" {
						isBlack = true
						break
					}
				}
			}
			if !isBlack {
				gameengine.SacrificePermanent(gs, p, "slaughter_pact_destroy")
				break
			}
		}
		break
	}

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: casterSeat,
		SourceCardName: "Slaughter Pact",
		EffectFn:       pactUpkeepPayOrLose(casterSeat, "Slaughter Pact", 3, 1, "B"),
	})

	emit(gs, slug, "Slaughter Pact", map[string]interface{}{
		"seat":    casterSeat,
		"delayed": "your_next_upkeep_pay_2B_or_lose",
	})
}

// registerInterventionPact wires up Intervention Pact.
//
// Oracle text:
//
//	The next time a source of your choice would deal damage to
//	you this turn, prevent that damage. You gain life equal to
//	the damage prevented this way.
//	At the beginning of your next upkeep, pay {1}{W}{W}. If you
//	don't, you lose the game.
func registerInterventionPact(r *Registry) {
	r.OnCast("Intervention Pact", interventionPactOnCast)
}

func interventionPactOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "intervention_pact_delayed"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Simplified: gain 5 life as approximation of damage prevention.
	if casterSeat >= 0 && casterSeat < len(gs.Seats) && gs.Seats[casterSeat] != nil {
		gameengine.GainLife(gs, casterSeat, 5, "Intervention Pact")
		gs.LogEvent(gameengine.Event{
			Kind:   "life_change",
			Seat:   casterSeat,
			Source: "Intervention Pact",
			Amount: 5,
			Details: map[string]interface{}{
				"from": gs.Seats[casterSeat].Life - 5,
				"to":   gs.Seats[casterSeat].Life,
			},
		})
	}

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: casterSeat,
		SourceCardName: "Intervention Pact",
		EffectFn:       pactUpkeepPayOrLose(casterSeat, "Intervention Pact", 3, 2, "W"),
	})

	emit(gs, slug, "Intervention Pact", map[string]interface{}{
		"seat":    casterSeat,
		"delayed": "your_next_upkeep_pay_1WW_or_lose",
	})
}

// registerSummonersPact wires up Summoner's Pact.
//
// Oracle text:
//
//	Search your library for a green creature card, reveal it, put
//	it into your hand, then shuffle.
//	At the beginning of your next upkeep, pay {2}{G}{G}. If you
//	don't, you lose the game.
func registerSummonersPact(r *Registry) {
	r.OnCast("Summoner's Pact", summonersPactOnCast)
}

func summonersPactOnCast(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "summoners_pact_delayed"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Search library for a green creature card and put it in hand.
	if casterSeat >= 0 && casterSeat < len(gs.Seats) {
		seat := gs.Seats[casterSeat]
		for _, c := range seat.Library {
			if c == nil {
				continue
			}
			isCreature := false
			isGreen := false
			for _, t := range c.Types {
				if t == "creature" {
					isCreature = true
				}
			}
			for _, col := range c.Colors {
				if col == "G" {
					isGreen = true
				}
			}
			if isCreature && isGreen {
				gameengine.MoveCard(gs, c, casterSeat, "library", "hand", "tutor-to-hand")
				gs.LogEvent(gameengine.Event{
					Kind:   "tutor",
					Seat:   casterSeat,
					Source: "Summoner's Pact",
					Details: map[string]interface{}{
						"card": c.DisplayName(),
						"from": "library",
						"to":   "hand",
					},
				})
				break
			}
		}
	}

	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_upkeep",
		ControllerSeat: casterSeat,
		SourceCardName: "Summoner's Pact",
		EffectFn:       pactUpkeepPayOrLose(casterSeat, "Summoner's Pact", 4, 2, "G"),
	})

	emit(gs, slug, "Summoner's Pact", map[string]interface{}{
		"seat":    casterSeat,
		"delayed": "your_next_upkeep_pay_2GG_or_lose",
	})
}

// pactUpkeepPayOrLose returns a delayed-trigger EffectFn that attempts
// to pay {totalCost} (with {colorCount} of {color}) at the controller's
// next upkeep. If the payment fails, the player loses the game.
//
// This is the shared pattern for all 5 Pact cards.
func pactUpkeepPayOrLose(casterSeat int, pactName string, totalCost, colorCount int, color string) func(gs *gameengine.GameState) {
	return func(gs *gameengine.GameState) {
		if casterSeat < 0 || casterSeat >= len(gs.Seats) {
			return
		}
		seat := gs.Seats[casterSeat]
		if seat == nil || seat.Lost {
			return
		}

		paid := false
		if seat.Mana != nil {
			// Check typed mana pool.
			colorAvail := typedManaColor(seat.Mana, color)
			if colorAvail >= colorCount && seat.Mana.Total() >= totalCost {
				// Pay colored portion.
				deductTypedMana(seat.Mana, color, colorCount)
				remaining := totalCost - colorCount
				// Pay generic from any available.
				drainGenericFromPool(seat.Mana, remaining)
				paid = true
			}
		} else if seat.ManaPool >= totalCost {
			seat.ManaPool -= totalCost
			gameengine.SyncManaAfterSpend(seat)
			paid = true
		}

		if !paid {
			seat.Lost = true
			seat.LossReason = "failed_to_pay_" + normalizeName(pactName)
			gs.LogEvent(gameengine.Event{
				Kind:   "lose_game",
				Seat:   casterSeat,
				Source: pactName,
				Details: map[string]interface{}{
					"reason": "failed_to_pay_pact",
					"rule":   "pact_trigger",
				},
			})
		} else {
			gs.LogEvent(gameengine.Event{
				Kind:   "mana_paid",
				Seat:   casterSeat,
				Source: pactName,
				Details: map[string]interface{}{
					"cost": totalCost,
					"rule": "pact_trigger",
				},
			})
		}
	}
}

// typedManaColor returns the amount of a specific color available in the pool.
func typedManaColor(m *gameengine.ColoredManaPool, color string) int {
	if m == nil {
		return 0
	}
	switch color {
	case "W":
		return m.W
	case "U":
		return m.U
	case "B":
		return m.B
	case "R":
		return m.R
	case "G":
		return m.G
	}
	return 0
}

// deductTypedMana subtracts the given amount from the specified color pool.
func deductTypedMana(m *gameengine.ColoredManaPool, color string, amount int) {
	if m == nil {
		return
	}
	switch color {
	case "W":
		m.W -= amount
	case "U":
		m.U -= amount
	case "B":
		m.B -= amount
	case "R":
		m.R -= amount
	case "G":
		m.G -= amount
	}
}

// drainGenericFromPool pays generic mana from whatever is available.
func drainGenericFromPool(m *gameengine.ColoredManaPool, amount int) {
	if m == nil || amount <= 0 {
		return
	}
	for amount > 0 {
		if m.C > 0 {
			m.C--
			amount--
		} else if m.W > 0 {
			m.W--
			amount--
		} else if m.U > 0 {
			m.U--
			amount--
		} else if m.B > 0 {
			m.B--
			amount--
		} else if m.R > 0 {
			m.R--
			amount--
		} else if m.G > 0 {
			m.G--
			amount--
		} else if m.Any > 0 {
			m.Any--
			amount--
		} else {
			break
		}
	}
}
