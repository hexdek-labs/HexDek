package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDramaticReversal wires up Dramatic Reversal.
//
// Oracle text:
//
//	Untap all nonland permanents you control.
//
// The cheap instant that pairs with Isochron Scepter. Oracle-text
// straightforward: untap every nonland permanent controlled by the
// caster. In the Scepter+Reversal loop, "all nonland permanents"
// includes Scepter itself, the mana rocks that funded the {2}
// activation, and Dockside Extortionist treasures — every source of
// the mana just spent is refreshed, giving another cycle.
//
// Batch #3 scope:
//   - OnResolve: iterate the caster's battlefield, untap every
//     nonland permanent. Emit untap events for each.
func registerDramaticReversal(r *Registry) {
	r.OnResolve("Dramatic Reversal", dramaticReversalResolve)
}

func dramaticReversalResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dramatic_reversal"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	untapped := 0
	for _, p := range s.Battlefield {
		if p == nil || p.IsLand() {
			continue
		}
		if !p.Tapped {
			continue
		}
		p.Tapped = false
		untapped++
		gs.LogEvent(gameengine.Event{
			Kind:   "untap",
			Seat:   seat,
			Target: seat,
			Source: item.Card.DisplayName(),
			Details: map[string]interface{}{
				"target_card": p.Card.DisplayName(),
				"reason":      "dramatic_reversal",
			},
		})
	}
	emit(gs, slug, item.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"untapped": untapped,
	})
}
