package gameengine

// keywords_councils_dilemma.go — Council's Dilemma (CR §701.20, Conspiracy:
// Take the Crown 2016) as a real multi-vote tally + scaling-effect fan-out.
//
// CR §701.20a: "Council's dilemma" instructs each player, in turn order
//               starting with the controller, to vote for one of the
//               named options. Unlike Will of the Council (a §701.38
//               winner-take-all vote), Council's Dilemma counts ALL
//               votes and lets the effect scale with the tally on
//               EACH option. Multiple players may vote for the same
//               option — votes stack.
//
// Worked example (Coercive Recruiter, Council's Dilemma — Pirate or
// Treasure):
//
//   - 4-player game; controller is seat 0.
//   - Vote order: 0 → 1 → 2 → 3 (controller-first, then turn order).
//   - 0 and 2 vote Pirate; 1 and 3 vote Treasure.
//   - Tally: {"Pirate": 2, "Treasure": 2}.
//   - Effect runs per option: "for each Pirate vote, take a creature"
//     (×2) AND "for each Treasure vote, make a Treasure token" (×2).
//   - Compare against Will of the Council: only the option with more
//     votes would trigger; ties break by the controller.
//
// Surface:
//
//   - TallyCouncilsDilemma(gs, controller, options, callback)
//       → map[string]int per-option vote counts
//   - ApplyCouncilsDilemma(gs, tally, effect)
//       → iterates non-zero options, invokes effect(option, votes)
//
// The callback is engine-agnostic — the Hat's preference model, a
// per-card snowflake selector, or a test fixture can all plug in. The
// engine just runs the seat iteration in the right order and the
// effect fan-out in tally order.

// ---------------------------------------------------------------------------
// TallyCouncilsDilemma
// ---------------------------------------------------------------------------

// CouncilsDilemmaVoter is the callback shape: given a seat and the slice
// of options being voted on, return the index of the option the seat
// votes for. Returning a negative index or an index >= len(options) is
// treated as an abstention (the seat's vote is not counted).
type CouncilsDilemmaVoter func(seat int, options []string) int

// TallyCouncilsDilemma runs a §701.20 Council's Dilemma vote and
// returns the per-option tally. Iteration order is "controller first,
// then turn order" (CR §701.20a) — for a 4-player table with
// controllerSeat=2, the order is 2 → 3 → 0 → 1.
//
// Lost / eliminated seats (seat.Lost == true) are skipped per CR
// §800.4 — a player who has left the game can't vote.
//
// Every option that appears in `options` shows up in the returned map,
// even ones with zero votes — callers iterating the map don't need
// special-case "missing key" handling. The map is also written to the
// log event so downstream observers see the full ballot.
//
// Returns nil when `gs`, `options`, or `callback` is nil/empty, or
// when controllerSeat is out of range.
func TallyCouncilsDilemma(gs *GameState, controllerSeat int, options []string, callback CouncilsDilemmaVoter) map[string]int {
	if gs == nil || callback == nil || len(options) == 0 {
		return nil
	}
	n := len(gs.Seats)
	if controllerSeat < 0 || controllerSeat >= n {
		return nil
	}
	tally := make(map[string]int, len(options))
	for _, opt := range options {
		tally[opt] = 0
	}
	// Iterate controllerSeat → controllerSeat+1 → ... in modular order.
	for offset := 0; offset < n; offset++ {
		seatIdx := (controllerSeat + offset) % n
		seat := gs.Seats[seatIdx]
		if seat == nil || seat.Lost {
			continue
		}
		idx := callback(seatIdx, options)
		if idx < 0 || idx >= len(options) {
			// Abstention.
			gs.LogEvent(Event{
				Kind: "vote_abstain",
				Seat: seatIdx,
				Details: map[string]interface{}{
					"reason": "callback_returned_invalid_index",
					"rule":   "701.38a",
				},
			})
			continue
		}
		chosen := options[idx]
		tally[chosen]++
		gs.LogEvent(Event{
			Kind: "vote",
			Seat: seatIdx,
			Details: map[string]interface{}{
				"controller_seat": controllerSeat,
				"option":          chosen,
				"option_index":    idx,
				"running_count":   tally[chosen],
				"rule":            "701.38a",
			},
		})
	}
	// Snapshot the final tally for log observers.
	finalSnapshot := make(map[string]interface{}, len(tally))
	for k, v := range tally {
		finalSnapshot[k] = v
	}
	gs.LogEvent(Event{
		Kind: "councils_dilemma_tally",
		Seat: controllerSeat,
		Details: map[string]interface{}{
			"options": append([]string(nil), options...),
			"tally":   finalSnapshot,
			"rule":    "701.38",
		},
	})
	return tally
}

// ---------------------------------------------------------------------------
// ApplyCouncilsDilemma
// ---------------------------------------------------------------------------

// ApplyCouncilsDilemma fans out the per-option scaling effect for a
// completed §701.20 tally. The effect callback fires ONCE per option
// that received a non-zero number of votes; zero-vote options are
// skipped so the caller can write `effect("Pirate", votes)` without
// having to guard against `votes == 0`.
//
// Iteration order matches the slice order the caller used when
// calling TallyCouncilsDilemma — the helper accepts an explicit
// options slice so the fan-out is deterministic even though
// map iteration in Go is randomized.
//
// `gs` is accepted for symmetry and so the helper can emit a
// completion event — the effect callback itself receives only the
// option name and the vote count, since the engine state is closed
// over by the caller's closure.
func ApplyCouncilsDilemma(gs *GameState, options []string, tally map[string]int, effect func(option string, votes int)) {
	if gs == nil || effect == nil || tally == nil {
		return
	}
	for _, opt := range options {
		votes := tally[opt]
		if votes <= 0 {
			continue
		}
		effect(opt, votes)
		gs.LogEvent(Event{
			Kind:   "councils_dilemma_effect",
			Source: opt,
			Amount: votes,
			Details: map[string]interface{}{
				"option": opt,
				"votes":  votes,
				"rule":   "701.38b",
			},
		})
	}
}
