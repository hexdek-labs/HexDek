package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMysticConfluence wires Mystic Confluence.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Mystic%20Confluence):
//
//	Choose three. You may choose the same mode more than once.
//	  • Counter target spell unless its controller pays {3}.
//	  • Return target creature to its owner's hand.
//	  • Draw a card.
//
// {3}{U}{U} Instant. Premier flex blue toolbox — counter (with a tax
// rider that mid-game opponents often pay), bounce, draw, all in one
// shell. Plays in cEDH control + late-game-resilient blue piles.
//
// Implementation:
//   - OnResolve picks 3 mode-tokens with possible repeats. Heuristic:
//     each mode is scored by current board state; the 3 highest scores
//     get executed (with replacement).
//   - **Counter unless {3}**: only fires if there's a counterable
//     opponent spell on the stack AND its controller can't (or won't)
//     pay {3}. Follow the same greedy "pay if affordable" policy as
//     Rhystic Study / Mystic Remora — opponent pays if ManaPool >= 3.
//     Score: 30 if counter resolves (cripples opp tempo), 0 if no spell
//     or opp pays.
//   - **Bounce creature**: only fires if there's a legal opponent
//     creature. Score = best creature's power (priority on big
//     attackers). Returns to owner's hand via BouncePermanent.
//   - **Draw**: always available, score 5 (steady value).
//
// Selection: pick 3 modes with the highest scores, ties broken in
// favor of card-advantage modes (draw > bounce > counter when all
// equal). Each mode is selected ONCE even with repeats — re-scoring
// after each selection so duplicates only happen when the next
// best score remains higher than another mode.
func registerMysticConfluence(r *Registry) {
	r.OnResolve("Mystic Confluence", mysticConfluenceResolve)
}

func mysticConfluenceResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "mystic_confluence"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	chosen := []string{}
	for i := 0; i < 3; i++ {
		mode := pickMysticConfluenceMode(gs, seat)
		switch mode {
		case "counter":
			target := findCounterableSpell(gs, seat, nil)
			if target == nil {
				// Fallback to draw if no target available.
				drawOne(gs, seat, "Mystic Confluence")
				chosen = append(chosen, "draw_fallback")
				continue
			}
			// Opponent may pay {3}.
			opp := gs.Seats[target.Controller]
			if opp != nil && opp.ManaPool >= 3 {
				opp.ManaPool -= 3
				gameengine.SyncManaAfterSpend(opp)
				gs.LogEvent(gameengine.Event{
					Kind:   "pay_mana",
					Seat:   target.Controller,
					Source: "Mystic Confluence",
					Amount: 3,
					Details: map[string]interface{}{
						"reason": "mystic_confluence_counter_tax",
					},
				})
				chosen = append(chosen, "counter_paid")
				// Fall through to draw as fallback for this mode slot —
				// the choice was the counter mode but it fizzled. To keep
				// the spell impactful, treat this as a no-op slot rather
				// than auto-drawing (would double-count).
				continue
			}
			target.Countered = true
			emitCounter(gs, slug, "Mystic Confluence", seat, target)
			chosen = append(chosen, "counter")
		case "bounce":
			best := pickMysticConfluenceBounceTarget(gs, seat)
			if best == nil {
				drawOne(gs, seat, "Mystic Confluence")
				chosen = append(chosen, "draw_fallback")
				continue
			}
			gameengine.BouncePermanent(gs, best, nil, "hand")
			chosen = append(chosen, "bounce")
		case "draw":
			drawOne(gs, seat, "Mystic Confluence")
			chosen = append(chosen, "draw")
		default:
			drawOne(gs, seat, "Mystic Confluence")
			chosen = append(chosen, "draw")
		}
	}

	emit(gs, slug, "Mystic Confluence", map[string]interface{}{
		"seat":  seat,
		"modes": chosen,
	})
}

func pickMysticConfluenceMode(gs *gameengine.GameState, seat int) string {
	counterScore := 0
	if target := findCounterableSpell(gs, seat, nil); target != nil {
		opp := gs.Seats[target.Controller]
		if opp == nil || opp.ManaPool < 3 {
			counterScore = 30
		} else {
			counterScore = 5 // forces opp to pay 3 — minor value
		}
	}
	bounceScore := 0
	if best := pickMysticConfluenceBounceTarget(gs, seat); best != nil {
		bounceScore = 10 + best.Power()
	}
	drawScore := 5

	if counterScore >= bounceScore && counterScore >= drawScore && counterScore > 0 {
		return "counter"
	}
	if bounceScore >= drawScore && bounceScore > 0 {
		return "bounce"
	}
	return "draw"
}

func pickMysticConfluenceBounceTarget(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if gs == nil {
		return nil
	}
	var best *gameengine.Permanent
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == seat {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if best == nil || p.Power() > best.Power() {
				best = p
			}
		}
	}
	return best
}
