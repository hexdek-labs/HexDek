package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTimeWarp wires Time Warp.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Time%20Warp):
//
//	Target player takes an extra turn after this one.
//
// {3}{U}{U} Sorcery. The original extra-turn template — five-mana Time
// Walk. The "target player" wording technically allows giving an opponent
// the extra turn (politics, or to dodge a "skip your next turn"
// effect), but the hat policy always self-targets in cEDH/EDH lines.
//
// Implementation:
//   - OnResolve. Default target is self; only override when an opponent
//     has a benign "skip-next-turn" effect pending — that decision lives
//     above this handler in the targeting layer, so the per_card path
//     here just respects item.Targets[0] if a player target was already
//     stamped during cast.
//   - Increments gs.Flags["extra_turns_pending"] — same primitive
//     resolveExtraTurn uses for AST-detected ExtraTurn nodes, so the
//     phase loop's extra-turn drain logic processes our entry exactly
//     like a Time Stretch or Capture of Jingzhou resolution.
//   - Self-target is the picker default; cross-targeting an opponent
//     emits an event with target_seat != caster_seat so logs show
//     intent.
func registerTimeWarp(r *Registry) {
	r.OnResolve("Time Warp", timeWarpResolve)
}

func timeWarpResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "time_warp"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	// Hat policy: self-target. The printed "target player" wording
	// allows giving the extra turn to an opponent (politics, dodge an
	// opp's "skip next turn" effect), but a per_card handler with no
	// negotiation context picks the dominant line every time, which is
	// always the caster.
	targetSeat := seat
	if gs.Seats[targetSeat] == nil || gs.Seats[targetSeat].Lost {
		emitFail(gs, slug, "Time Warp", "controller_lost", nil)
		return
	}

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["extra_turns_pending"]++
	// Some phase implementations also track per-seat extra-turn ownership.
	// Mirror that on the targeted seat's flags for handlers that gate on
	// "did I just get an extra turn from someone else" (Brago Eternal,
	// Notion Thief, etc.) — the seat-level marker is consumed lazily by
	// the phase loop and is no-op when absent.
	if gs.Seats[targetSeat] != nil {
		if gs.Seats[targetSeat].Flags == nil {
			gs.Seats[targetSeat].Flags = map[string]int{}
		}
		gs.Seats[targetSeat].Flags["extra_turn_queued"]++
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "extra_turn",
		Seat:   targetSeat,
		Source: "Time Warp",
	})
	emit(gs, slug, "Time Warp", map[string]interface{}{
		"seat":        seat,
		"target_seat": targetSeat,
		"pending":     gs.Flags["extra_turns_pending"],
	})
}
