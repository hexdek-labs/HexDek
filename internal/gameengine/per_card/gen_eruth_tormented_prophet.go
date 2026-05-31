package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEruthTormentedProphet wires Eruth, Tormented Prophet.
//
// Oracle text:
//
//	If you would draw a card, exile the top two cards of your library
//	instead. You may play those cards this turn.
//
// Implementation (R46 stub port):
//   - OnETB registers a `would_draw` replacement scoped to the
//     controller. Apply cancels the draw, exiles up to two cards from
//     the top of the controller's library, and grants each one a
//     `until_end_of_turn` ZoneCastPermission from exile at normal
//     mana cost.
//   - SourcePerm = Eruth, so the engine's
//     UnregisterReplacementsForPermanent cleans up on LTB without a
//     dedicated hook here.
func registerEruthTormentedProphet(r *Registry) {
	r.OnETB("Eruth, Tormented Prophet", eruthRegisterDrawReplacement)
}

func eruthRegisterDrawReplacement(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eruth_would_draw_exile_two"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	gs.RegisterReplacement(&gameengine.ReplacementEffect{
		EventType:      "would_draw",
		HandlerID:      "Eruth, Tormented Prophet:exile_two:" + strconv.Itoa(perm.Timestamp),
		SourcePerm:     perm,
		ControllerSeat: controller,
		Timestamp:      perm.Timestamp,
		Category:       gameengine.CategoryOther,
		Applies: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) bool {
			if ev == nil {
				return false
			}
			return ev.TargetSeat == controller && ev.Count() > 0 && !ev.Cancelled
		},
		ApplyFn: func(gs *gameengine.GameState, ev *gameengine.ReplEvent) {
			ev.Cancelled = true
			seat := gs.Seats[controller]
			if seat == nil {
				return
			}
			exiled := 0
			for i := 0; i < 2 && len(seat.Library) > 0; i++ {
				top := seat.Library[0]
				if top == nil {
					seat.Library = seat.Library[1:]
					continue
				}
				gameengine.MoveCard(gs, top, controller, "library", "exile", "eruth_draw_replacement_exile")
				gameengine.RegisterZoneCastGrant(gs, top, &gameengine.ZoneCastPermission{
					Zone:              gameengine.ZoneExile,
					Keyword:           "eruth_play_this_turn",
					ManaCost:          -1,
					RequireController: controller,
					SourceName:        "Eruth, Tormented Prophet",
					Duration:          "until_end_of_turn",
					SourceTimestamp:   perm.Timestamp,
					GrantTurn:         gs.Turn,
				})
				exiled++
			}
			gs.LogEvent(gameengine.Event{
				Kind:   "replacement_applied",
				Seat:   controller,
				Source: "Eruth, Tormented Prophet",
				Amount: exiled,
				Details: map[string]interface{}{
					"slug":   slug,
					"rule":   "614",
					"effect": "draw_to_exile_top_two",
				},
			})
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     controller,
		"replaces": "would_draw",
	})
}
