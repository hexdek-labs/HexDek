package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSundialOfTheInfinite wires up Sundial of the Infinite.
//
// Oracle text:
//
//	{1}, {T}: End the turn. Activate only during your turn.
//
// Niche but powerful: ends the turn immediately after main phase,
// sidestepping end-of-turn triggers (for better AND worse). Iconic
// interactions:
//
//   - Final Fortune / Last Chance: "Take an extra turn. At the
//     beginning of that turn's end step, you lose the game." —
//     Sundial cuts the end step, skipping the lose trigger.
//   - Nexus of Fate / Beacon of Tomorrows: stop opponents' end-of-
//     turn effects from firing during the extra-turn window.
//   - Tergrid 3-land line: Sundial skips sacrifice-at-end triggers.
//
// Batch #2 scope:
//   - OnActivated(0, ...): set gs.Flags["turn_ending_now"] = 1 and
//     clear pending delayed triggers for the current turn. The phase
//     loop (phases.go) consumes the flag and short-circuits to the
//     cleanup step.
//
// We can't directly fast-forward the phase loop from here — phases.go
// controls that — but setting the flag gives the phase loop a hook.
// Downstream work in phases.go will implement the actual short-circuit.
// For tests, we log per_card_handler and the flag so callers can
// observe the activation.
func registerSundialOfTheInfinite(r *Registry) {
	r.OnActivated("Sundial of the Infinite", sundialActivate)
}

func sundialActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "sundial_end_turn"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller

	// "Activate only during your turn" — CR restriction on Sundial.
	if gs.Active != seat {
		emitFail(gs, slug, src.Card.DisplayName(), "not_your_turn", map[string]interface{}{
			"active_seat":     gs.Active,
			"controller_seat": seat,
		})
		return
	}

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Signal to tournament/turn.go: fast-forward to cleanup.
	gs.Flags["turn_ending_now"] = 1
	gs.Flags["sundial_end_turn_seat_"+intToStr(seat)] = 1
	// Cancel any delayed triggers that would fire in the remainder of
	// this turn (end_of_turn, next_end_step). "End the turn" per CR
	// §712.5c: "Any abilities that would trigger during or because of
	// the remainder of the turn are no longer triggered."
	removed := 0
	var kept []*gameengine.DelayedTrigger
	for _, dt := range gs.DelayedTriggers {
		if dt == nil {
			continue
		}
		if dt.TriggerAt == "end_of_turn" || dt.TriggerAt == "next_end_step" || dt.TriggerAt == "end_of_combat" {
			removed++
			continue
		}
		kept = append(kept, dt)
	}
	gs.DelayedTriggers = kept
	// CR §712.5c: "all spells and abilities on the stack are exiled"
	// (not just discarded). Move each spell's card to exile zone.
	stackDropped := len(gs.Stack)
	for _, item := range gs.Stack {
		if item == nil {
			continue
		}
		if item.Card != nil {
			exileSeat := item.Controller
			if exileSeat >= 0 && exileSeat < len(gs.Seats) && gs.Seats[exileSeat] != nil {
				gameengine.MoveCard(gs, item.Card, exileSeat, "stack", "exile", "sundial-end-turn")
			}
		}
	}
	gs.Stack = nil
	gs.LogEvent(gameengine.Event{
		Kind:   "end_turn_now",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":                  "712.5c",
			"delayed_triggers_lost": removed,
			"stack_items_dropped":   stackDropped,
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":                seat,
		"delayed_removed":     removed,
		"stack_items_dropped": stackDropped,
	})
}
