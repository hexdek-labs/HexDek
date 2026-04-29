package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGriselbrand wires up Griselbrand.
//
// Oracle text:
//
//	Flying, lifelink
//	Pay 7 life: Draw seven cards.
//
// The quintessential reanimator target. Animate Dead / Reanimate /
// Exhume / Shallow Grave / Necromancy → Griselbrand on turn 2-3 →
// pay 7 life, draw 7 → pay 7 life again (his lifelink triggers from
// combat refund the life over time; more commonly the draw fuels a
// win the same turn via Tendrils / Thoracle).
//
// Batch #3 scope:
//   - OnActivated(0, ...): pay 7 life, draw 7 cards. Iterative —
//     each draw logs separately so Ad Nauseam-style counters tick.
//     If the library has fewer than 7 cards, we draw all available
//     and set AttemptedEmptyDraw for §704.5b SBA consumption.
//
// Static abilities (flying/lifelink) are keyword-handled upstream;
// the per_card handler only wires the activated draw.
func registerGriselbrand(r *Registry) {
	r.OnActivated("Griselbrand", griselbrandActivate)
}

func griselbrandActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "griselbrand_pay_seven_draw_seven"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	s := gs.Seats[seat]
	// Pay 7 life (caller didn't — we enforce the cost for correctness).
	s.Life -= 7
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   seat,
		Target: seat,
		Source: src.Card.DisplayName(),
		Amount: 7,
		Details: map[string]interface{}{
			"reason": "griselbrand_activation_cost",
		},
	})
	// Draw 7.
	drew := 0
	for i := 0; i < 7; i++ {
		if len(s.Library) == 0 {
			s.AttemptedEmptyDraw = true
			gs.LogEvent(gameengine.Event{
				Kind:   "draw_failed_empty",
				Seat:   seat,
				Source: src.Card.DisplayName(),
			})
			break
		}
		c := s.Library[0]
		gameengine.MoveCard(gs, c, seat, "library", "hand", "draw")
		drew++
		gs.LogEvent(gameengine.Event{
			Kind:   "draw",
			Seat:   seat,
			Target: seat,
			Source: src.Card.DisplayName(),
			Amount: 1,
		})
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":       seat,
		"life_paid":  7,
		"life_after": s.Life,
		"cards_drawn": drew,
	})
	_ = gs.CheckEnd()
}
