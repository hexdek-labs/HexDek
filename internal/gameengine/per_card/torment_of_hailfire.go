package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTormentOfHailfire wires up Torment of Hailfire.
//
// Oracle text:
//
//	Repeat the following process X times. Each opponent loses 3 life
//	unless that player sacrifices a nonland permanent or discards a
//	card.
//
// Implementation:
//   - OnResolve: for X iterations, each opponent either sacrifices a
//     nonland permanent, discards a card, or loses 3 life (in that
//     priority order, simulating rational play).
func registerTormentOfHailfire(r *Registry) {
	r.OnResolve("Torment of Hailfire", tormentOfHailfireResolve)
}

func tormentOfHailfireResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "torment_of_hailfire"
	if gs == nil || item == nil {
		return
	}
	casterSeat := item.Controller

	// Determine X. Torment is {X}{B}{B}, so X = CMC - 2.
	// The simulation doesn't track X explicitly on StackItem, so we
	// derive it from the card's CMC or from the caster's remaining mana.
	xVal := 0
	if item.Card != nil && item.Card.CMC > 2 {
		xVal = item.Card.CMC - 2
	}
	if xVal <= 0 {
		// Fallback: use however much mana the caster has available,
		// minus 2 for the BB cost.
		if casterSeat >= 0 && casterSeat < len(gs.Seats) {
			pool := gs.Seats[casterSeat].ManaPool
			if pool > 2 {
				xVal = pool - 2
			}
		}
	}
	if xVal <= 0 {
		xVal = 3 // Reasonable default for simulation.
	}

	opps := gs.Opponents(casterSeat)
	totalLifeLost := map[int]int{}
	totalSacrificed := map[int]int{}
	totalDiscarded := map[int]int{}

	for i := 0; i < xVal; i++ {
		for _, oppIdx := range opps {
			opp := gs.Seats[oppIdx]
			if opp == nil || opp.Lost {
				continue
			}

			// Priority 1: sacrifice a nonland permanent.
			sacrificed := false
			for j := len(opp.Battlefield) - 1; j >= 0; j-- {
				p := opp.Battlefield[j]
				if p == nil || p.IsLand() {
					continue
				}
				gameengine.SacrificePermanent(gs, p, "torment_of_hailfire")
				totalSacrificed[oppIdx]++
				sacrificed = true
				break
			}
			if sacrificed {
				continue
			}

			// Priority 2: discard a card.
			if len(opp.Hand) > 0 {
				discarded := opp.Hand[len(opp.Hand)-1]
				gameengine.DiscardCard(gs, discarded, oppIdx)
				totalDiscarded[oppIdx]++
				gs.LogEvent(gameengine.Event{
					Kind:   "discard",
					Seat:   oppIdx,
					Source: "Torment of Hailfire",
					Details: map[string]interface{}{
						"card": discarded.DisplayName(),
					},
				})
				continue
			}

			// Priority 3: lose 3 life.
			opp.Life -= 3
			totalLifeLost[oppIdx] += 3
			gs.LogEvent(gameengine.Event{
				Kind:   "life_change",
				Seat:   oppIdx,
				Source: "Torment of Hailfire",
				Amount: -3,
				Details: map[string]interface{}{
					"from": opp.Life + 3,
					"to":   opp.Life,
				},
			})
		}
	}

	emit(gs, slug, "Torment of Hailfire", map[string]interface{}{
		"seat":       casterSeat,
		"x":          xVal,
		"life_lost":  totalLifeLost,
		"sacrificed": totalSacrificed,
		"discarded":  totalDiscarded,
	})
}
