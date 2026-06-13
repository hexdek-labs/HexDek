package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// time_stretch_r60.go — per_card handler for Time Stretch.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Target player takes two extra turns after this one.
//
// {8}{U}{U} Sorcery. The premier "double Time Walk" — eight-plus-mana
// finisher that hands the caster two consecutive extra turns. Parses to
// a `parsed_effect_residual` raw-text node ("target player takes two
// extra turns after this one") with no structured ExtraTurn node, so the
// generic AST dispatch logs the clause inert and grants ZERO extra turns
// — the entire payoff of the card was dead in production. Its sister
// card Time Warp already has a per_card handler (time_warp.go) using the
// exact same `extra_turns_pending` primitive; this is the n=2 twin.
//
// Implementation:
//   - OnResolve. Hat policy self-targets (the "target player" wording
//     allows gifting an opponent the turns, but a per_card handler with
//     no negotiation context always takes the dominant line, which is
//     the caster — identical reasoning to Time Warp).
//   - Increments gs.Flags["extra_turns_pending"] by 2 (the same
//     primitive resolveExtraTurn / Time Warp use), and bumps the
//     targeted seat's extra_turn_queued marker by 2 so handlers that
//     gate on "did I just get an extra turn" observe both.
func init() {
	registerTimeStretchR60(Global())
	AddResetHook(registerTimeStretchR60)
}

func registerTimeStretchR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Time Stretch", timeStretchResolve)
}

func timeStretchResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "time_stretch"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	targetSeat := seat
	if gs.Seats[targetSeat] == nil || gs.Seats[targetSeat].Lost {
		emitFail(gs, slug, "Time Stretch", "controller_lost", nil)
		return
	}

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["extra_turns_pending"] += 2

	if gs.Seats[targetSeat] != nil {
		if gs.Seats[targetSeat].Flags == nil {
			gs.Seats[targetSeat].Flags = map[string]int{}
		}
		gs.Seats[targetSeat].Flags["extra_turn_queued"] += 2
	}

	for i := 0; i < 2; i++ {
		gs.LogEvent(gameengine.Event{
			Kind:   "extra_turn",
			Seat:   targetSeat,
			Source: "Time Stretch",
		})
	}
	emit(gs, slug, "Time Stretch", map[string]interface{}{
		"seat":        seat,
		"target_seat": targetSeat,
		"granted":     2,
		"pending":     gs.Flags["extra_turns_pending"],
	})
}
