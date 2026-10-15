package gameengine

// keywords_will_of_council.go — Will of the Council (CR §701.18,
// Conspiracy 2014) two-option voting.
//
// Printed pattern:
//
//   "Will of the council — Starting with you, each player votes for
//    [option A] or [option B]. [Effect tied to whichever option got
//    more votes.]"
//
// CR §701.18a: When a Will of the Council ability instructs each player
//              to vote, voting proceeds in turn order starting with the
//              controller of the spell or ability. Each player votes
//              for one of the listed options.
// CR §701.18b: The card text specifies how ties are resolved (some
//              cards split the effect across both options, others say
//              "if the vote is tied, [option A] gets one more vote",
//              etc.). When the printed text is silent on ties, the
//              controller of the resolving spell/ability decides — but
//              by the Conspiracy 2014 / Commander Legends convention
//              the default for "split" wording is that NEITHER option
//              wins outright and the resolver picks how to apply both.
//
// This file provides the generic tally machinery — TallyWillOfCouncil
// returns the winner string ("" on a tie) and the per-option tally map
// so the resolver (the per_card handler implementing a specific WotC
// card) can branch on majority or apply tie-specific behavior.
//
// API shape:
//
//   winner, tally := TallyWillOfCouncil(
//       gs, controllerSeat,
//       "carnage", "homage",
//       func(seat int, options [2]string) int { /* ... */ },
//   )
//
// The callback receives the seat being polled and the two option
// strings (so seat-aware AI can reason about which option benefits
// whom); it returns 0 for optionA or 1 for optionB. Any other return
// is treated as an abstention (no vote counted for that seat — but
// the printed Will of the Council pattern doesn't admit abstentions
// in practice; callers shouldn't return out-of-range values).
//
// If voteCallback is nil, each seat votes uniformly at random via
// gs.Rng (deterministic given the engine's seeded RNG). This is the
// "unscripted" default the task specifies — sufficient for stress
// tests, Loki fuzz runs, and engine-level smoke tests that don't care
// about strategic vote choice.

import (
	"math/rand"
)

// WillOfCouncilVote is the per-seat decision callback. Returns 0 for
// optionA, 1 for optionB. Returns >= 2 to abstain (no vote tallied),
// though printed Will of the Council cards do not allow abstentions —
// callers should normally return 0 or 1.
type WillOfCouncilVote func(seat int, options [2]string) int

// TallyWillOfCouncil runs the voting round for a Will of the Council
// spell or ability controlled by `controllerSeat`. CR §701.18.
//
// Voting order: controllerSeat first, then living opponents in
// turn-order from the controller's left (the §101.4 convention used
// throughout the engine — same as Tempting Offer / Council's Judgment).
//
// Tie behavior: by default a tie returns winner == "". The tally map
// always carries both option counts, so the resolver can either:
//   - branch on winner != "" for a clear majority
//   - inspect the tally and apply card-specific tie rules (split effect,
//     "controller chooses", "neither effect happens", etc.)
//
// Returns:
//   - winner: optionA, optionB, or "" if the vote is tied
//   - tally:  map[optionA]countA, optionB]countB (always non-nil)
//
// Side effects: emits a "will_of_council_vote" event per voter (Seat
// = voter, Details["choice"] = chosen option), and a final
// "will_of_council_result" event with the winner and full tally.
func TallyWillOfCouncil(
	gs *GameState,
	controllerSeat int,
	optionA, optionB string,
	voteCallback WillOfCouncilVote,
) (winner string, tally map[string]int) {
	tally = map[string]int{optionA: 0, optionB: 0}
	if gs == nil {
		return "", tally
	}
	if controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return "", tally
	}
	if gs.Seats[controllerSeat] == nil {
		return "", tally
	}

	// Build voting order: controller first, then living opponents in
	// turn-order from controller's left. CR §101.4a anchor: when "each
	// player" is asked to make a choice "starting with you," APNAP from
	// the controller gives the canonical order.
	order := []int{controllerSeat}
	order = append(order, gs.LivingOpponents(controllerSeat)...)
	// Defensive: if the controller has been eliminated since the spell
	// was cast, drop them from the head of the order. The §701.18 text
	// says "Starting with you" but if "you" no longer exist there's
	// nothing to start with — the resolver should still tally surviving
	// players' votes.
	if gs.Seats[controllerSeat].Lost {
		order = order[1:]
	}

	options := [2]string{optionA, optionB}

	// Default callback: uniform-random via gs.Rng if no callback is
	// supplied. Use the engine RNG so reproducibility is preserved when
	// the caller has seeded the game.
	cb := voteCallback
	if cb == nil {
		rng := gs.Rng
		if rng == nil {
			// Last-resort: a private RNG seeded from time. Engine-driven
			// games always set gs.Rng; this branch is purely defensive
			// against unit-test scaffolds that forget to seed.
			rng = rand.New(rand.NewSource(1))
		}
		cb = func(seat int, opts [2]string) int {
			return rng.Intn(2)
		}
	}

	for _, seat := range order {
		choice := cb(seat, options)
		if choice < 0 || choice > 1 {
			gs.LogEvent(Event{
				Kind: "will_of_council_abstain",
				Seat: seat,
				Details: map[string]interface{}{
					"rule":       "701.38a",
					"controller": controllerSeat,
				},
			})
			continue
		}
		chosen := options[choice]
		tally[chosen]++
		gs.LogEvent(Event{
			Kind: "will_of_council_vote",
			Seat: seat,
			Details: map[string]interface{}{
				"choice":     chosen,
				"option_a":   optionA,
				"option_b":   optionB,
				"controller": controllerSeat,
				"rule":       "701.38a",
			},
		})
	}

	switch {
	case tally[optionA] > tally[optionB]:
		winner = optionA
	case tally[optionB] > tally[optionA]:
		winner = optionB
	default:
		winner = "" // tie — caller applies card-specific tie behavior
	}

	gs.LogEvent(Event{
		Kind:   "will_of_council_result",
		Seat:   controllerSeat,
		Source: winner,
		Details: map[string]interface{}{
			"option_a":  optionA,
			"option_b":  optionB,
			"votes_a":   tally[optionA],
			"votes_b":   tally[optionB],
			"winner":    winner,
			"tied":      winner == "",
			"voter_count": len(order),
			"rule":      "701.38b",
		},
	})

	return winner, tally
}
