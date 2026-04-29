package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTaintedPact wires up Tainted Pact.
//
// Oracle text:
//
//	Exile the top card of your library. You may repeat this process
//	any number of times. For each card exiled this way with the same
//	name as another card exiled this way, Tainted Pact has no effect
//	on that card, but those cards still go to exile. Stop when you
//	choose to stop or when you exile a card with the same name as
//	another card exiled this way.
//
// Thoracle-combo usage: with a singleton deck (99+ commander unique
// names), no duplicates exist, so this empties the library. Library
// empty → Thoracle wins.
//
// Policy MVP: always exile until duplicate (the combo line). A
// non-combo caster might want to stop earlier, but we don't expose a
// choice yet.
func registerTaintedPact(r *Registry) {
	r.OnResolve("Tainted Pact", taintedPactResolve)
}

func taintedPactResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "tainted_pact"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	seen := map[string]bool{}
	exiled := 0
	duplicateHit := false
	for len(s.Library) > 0 {
		c := s.Library[0]
		if seen[c.DisplayName()] {
			// Exile the duplicate and STOP.
			gameengine.MoveCard(gs, c, seat, "library", "exile", "exile-from-library")
			exiled++
			duplicateHit = true
			break
		}
		seen[c.DisplayName()] = true
		gameengine.MoveCard(gs, c, seat, "library", "exile", "exile-from-library")
		exiled++
	}
	emit(gs, slug, "Tainted Pact", map[string]interface{}{
		"seat":              seat,
		"exiled":            exiled,
		"duplicate_hit":     duplicateHit,
		"library_remaining": len(s.Library),
	})
}
