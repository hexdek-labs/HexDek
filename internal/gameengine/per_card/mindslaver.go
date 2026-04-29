package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMindslaver wires up Mindslaver.
//
// Oracle text:
//
//	{4}, {T}, Sacrifice Mindslaver: You control target player
//	during that player's next turn. (You see all cards that player
//	could see and make all decisions for the player.)
//
// Implementation:
//   - OnActivated: Pay {4}, tap, sacrifice. Set a delayed trigger that
//     fires at the target's next untap step and sets
//     target.ControlledBy = controller.
//   - The turn loop checks Seat.ControlledBy at decision points and
//     routes Hat calls to the controlling player's Hat.
//   - At the end of the controlled turn, ControlledBy resets to -1.
func registerMindslaver(r *Registry) {
	r.OnActivated("Mindslaver", mindslaverActivate)
}

func mindslaverActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mindslaver_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	// Must be untapped.
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}

	// Pick a target opponent. MVP: first living opponent.
	var target int = -1
	for _, opp := range gs.Opponents(seat) {
		target = opp
		break
	}
	if target < 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_valid_target", nil)
		return
	}

	// Tap + sacrifice via SacrificePermanent for proper zone-change handling:
	// replacement effects, dies/LTB triggers, commander redirect.
	src.Tapped = true
	gameengine.SacrificePermanent(gs, src, "mindslaver_activation")

	// Register a delayed trigger for the target's next turn.
	controllerSeat := seat
	targetSeat := target
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "your_next_turn",
		ControllerSeat: targetSeat, // fires on the TARGET's next untap
		SourceCardName: "Mindslaver",
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if targetSeat < 0 || targetSeat >= len(gs.Seats) {
				return
			}
			gs.Seats[targetSeat].ControlledBy = controllerSeat
			gs.LogEvent(gameengine.Event{
				Kind:   "mindslaver_control_start",
				Seat:   controllerSeat,
				Target: targetSeat,
				Details: map[string]interface{}{
					"rule": "712.6",
				},
			})
		},
	})

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"controller": controllerSeat,
		"target":     targetSeat,
	})
}
