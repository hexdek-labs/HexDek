package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// tashas_hideous_laughter_r60.go — per_card handler for Tasha's Hideous
// Laughter.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Each opponent exiles cards from the top of their library until that
//	player has exiled cards with total mana value 20 or greater.
//
// {1}{U}{U} Sorcery. A premier mill / exile-the-library payoff (the
// "mill ~7-9 cards from each opponent at once" engine). Parses to a
// `parsed_tail` raw-text node with no structured exile/until loop, so
// the generic dispatch logged it inert and exiled ZERO cards from
// anyone.
//
// Implementation:
//   - OnResolve. Iterates every living opponent, exiling the top card of
//     their library one at a time and accumulating exiled mana value
//     until the running total reaches 20 (CR rounding: the LAST card
//     pushes the total to 20+, then the loop stops), or the library
//     empties.
//   - Each move routes through MoveCard("library" -> "exile") so the
//     normal zone-change machinery (and any "leaves library" / "is
//     exiled" payoffs) fires per card.
func init() {
	registerTashasHideousLaughterR60(Global())
	AddResetHook(registerTashasHideousLaughterR60)
}

func registerTashasHideousLaughterR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Tasha's Hideous Laughter", tashasHideousLaughterResolve)
}

func tashasHideousLaughterResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "tashas_hideous_laughter"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	totalExiled := 0
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		mv := 0
		exiled := 0
		for mv < 20 && len(os.Library) > 0 {
			c := os.Library[0]
			cmc := 0
			if c != nil {
				cmc = c.CMC
			}
			gameengine.MoveCard(gs, c, opp, "library", "exile", "tashas-hideous-laughter")
			mv += cmc
			exiled++
		}
		totalExiled += exiled
		gs.LogEvent(gameengine.Event{
			Kind:   "exile",
			Seat:   seat,
			Target: opp,
			Source: "Tasha's Hideous Laughter",
			Amount: exiled,
			Details: map[string]interface{}{
				"exiled_mana_value": mv,
			},
		})
	}
	emit(gs, slug, "Tasha's Hideous Laughter", map[string]interface{}{
		"seat":         seat,
		"total_exiled": totalExiled,
	})
}
