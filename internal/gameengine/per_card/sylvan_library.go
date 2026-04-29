package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSylvanLibrary wires up Sylvan Library.
//
// Oracle text:
//
//   At the beginning of your draw step, you may draw two additional
//   cards. If you do, choose two cards in your hand drawn this turn.
//   For each of those cards, pay 4 life or put the card on top of
//   your library.
//
// GG enchantment. One of the best card-advantage enchantments in green.
// In cEDH, pilots often pay 8 life to keep both extra draws (especially
// at 40 life in commander).
//
// MVP implementation:
//   - OnTrigger "draw_step_controller": draw 2 extra cards.
//   - Heuristic: if life > 12, pay 4 life per card to keep both.
//     Otherwise put both back on top.
func registerSylvanLibrary(r *Registry) {
	r.OnTrigger("Sylvan Library", "draw_step_controller", sylvanLibraryDraw)
}

func sylvanLibraryDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sylvan_library_draw"
	if gs == nil || perm == nil {
		return
	}
	// Only fires during controller's draw step.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]

	// Draw two additional cards.
	var drawnCards []*gameengine.Card
	for i := 0; i < 2; i++ {
		c := drawOne(gs, seat, "Sylvan Library")
		if c != nil {
			drawnCards = append(drawnCards, c)
		}
	}
	if len(drawnCards) == 0 {
		emit(gs, slug, "Sylvan Library", map[string]interface{}{
			"seat":  seat,
			"drawn": 0,
		})
		return
	}

	// MVP heuristic: if life > 12, pay 4 life per drawn card to keep them.
	// Otherwise put them back on top of library.
	lifeCost := 4 * len(drawnCards)
	keepCards := s.Life > 12

	if keepCards {
		s.Life -= lifeCost
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   seat,
			Target: seat,
			Source: "Sylvan Library",
			Amount: lifeCost,
			Details: map[string]interface{}{
				"reason": "sylvan_library_keep_cards",
				"cards":  len(drawnCards),
			},
		})
		emit(gs, slug, "Sylvan Library", map[string]interface{}{
			"seat":      seat,
			"drawn":     len(drawnCards),
			"kept":      true,
			"life_paid": lifeCost,
			"life_now":  s.Life,
		})
	} else {
		// Put both cards back on top of library (in drawn order). Route
		// through MoveCard so §614 replacements and hand-leave triggers
		// observe the tuck.
		for i := len(drawnCards) - 1; i >= 0; i-- {
			c := drawnCards[i]
			gameengine.MoveCard(gs, c, seat, "hand", "library_top", "tuck-top")
		}
		emit(gs, slug, "Sylvan Library", map[string]interface{}{
			"seat":      seat,
			"drawn":     len(drawnCards),
			"kept":      false,
			"returned":  len(drawnCards),
		})
	}
	_ = gs.CheckEnd()
}
