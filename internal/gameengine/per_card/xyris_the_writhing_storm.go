package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerXyrisTheWrithingStorm wires Xyris, the Writhing Storm.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying
//	Whenever an opponent draws a card except the first one they draw in
//	each of their draw steps, create a 1/1 green Snake creature token.
//
// Implementation:
//   - Flying is handled by the AST keyword pipeline.
//   - OnTrigger "player_would_draw": fires before every draw event
//     (FireDrawTriggerObservers in cast_counts.go). We filter to:
//     (a) draw_seat is an opponent of Xyris's controller (not the
//         controller themselves),
//     (b) the draw is NOT the first mandatory draw-step draw for that
//         opponent — identified by `is_draw_step_draw == true` in the
//         event context (the `_suppress_first_draw_trigger_seat` flag
//         consumed by FireDrawTriggerObservers and passed here as a
//         bool).
//   - On each qualifying draw, one 1/1 green Snake creature token is
//     created under Xyris's controller via gameengine.CreateCreatureToken.
//
// Coverage gap: draws that bypass FireDrawTriggerObservers entirely
// (e.g. direct library→hand moves not routed through the standard draw
// path) will not fire player_would_draw and therefore will not trigger
// this handler. emitPartial flags the gap on ETB.
//
// CR note: "the first one they draw in each of their draw steps" means
// the mandatory turn-draw in the active player's draw step. Additional
// draws during the draw step (e.g. from Sylvan Library) are NOT exempt.
// is_draw_step_draw is true only for that single first draw-step draw,
// which exactly matches the oracle wording.
func registerXyrisTheWrithingStorm(r *Registry) {
	r.OnETB("Xyris, the Writhing Storm", xyrisTheWrithingStormETB)
	r.OnTrigger("Xyris, the Writhing Storm", "player_would_draw", xyrisTheWrithingStormDraw)
}

func xyrisTheWrithingStormETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "xyris_writhing_storm_coverage_gap"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"draws_not_routed_through_fire_draw_trigger_observers_not_tracked")
}

func xyrisTheWrithingStormDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "xyris_writhing_storm_snake_token"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	drawSeat, ok := ctx["draw_seat"].(int)
	if !ok || drawSeat < 0 || drawSeat >= len(gs.Seats) {
		return
	}

	// Only trigger on opponent draws — Xyris's controller drawing does not
	// create Snake tokens.
	if drawSeat == perm.Controller {
		return
	}

	// Skip the first mandatory draw of each opponent's draw step.
	// FireDrawTriggerObservers sets is_draw_step_draw = true exactly for
	// that single first draw-step draw and false for all other draws.
	isDrawStepDraw, _ := ctx["is_draw_step_draw"].(bool)
	if isDrawStepDraw {
		return
	}

	// The drawing opponent must still be in the game.
	drawerSeat := gs.Seats[drawSeat]
	if drawerSeat == nil || drawerSeat.Lost {
		return
	}

	// Create a 1/1 green Snake creature token under Xyris's controller.
	token := gameengine.CreateCreatureToken(
		gs,
		perm.Controller,
		"Snake Token",
		[]string{"creature", "snake", "pip:G"},
		1, 1,
	)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"drawer_seat": drawSeat,
		"token":       "Snake Token",
		"token_created": token != nil,
	})
}
