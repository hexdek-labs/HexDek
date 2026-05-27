package hat

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// vote.go — hat-side decision primitives for CR §701.32 voting cards
// (Council's Judgment / Magister of Worth / Coercive Portal / Custodi
// Lich / will-of-council uncommons). The engine ships the tally
// machinery in keywords_will_of_council.go (CR §701.18) and
// keywords_councils_dilemma.go (CR §701.20) with callback types
// `WillOfCouncilVote` and `CouncilsDilemmaVoter`, but neither has a
// production caller yet and the hat has no vote-decision method.
//
// These primitives are deliberately independent of the engine
// callback signatures — the engine APIs pass `(seat, options []string)`
// without per-option seat context, so per-card handlers (when they
// arrive for Council's Judgment / Custodi Lich / Coercive Portal)
// will do the option→seat mapping locally and consult these helpers
// for the actual decision.

// MostThreateningOpponent returns the seat index of our most-threatening
// living opponent, or -1 when none exist (e.g. all opponents are Lost /
// LeftGame, or only one seat is in the game).
//
// Threat read order:
//   1. opponentProfiles[i].ThreatLevel — the existing 3rd Eye composite
//      score updated each classifyOpponent call. Captures archetype
//      pressure (combo near win, aggro with wide board) layered on top
//      of raw board power.
//   2. Fallback to effectiveOffensivePower(gs, opp) — the evasion-
//      weighted summoning-sick-discounted offensive power used by
//      scoreThreat. Fires when no profile exists or all profiles read 0.
//
// Both passes ignore Lost / LeftGame seats per CR §800.4a.
func (h *YggdrasilHat) MostThreateningOpponent(gs *gameengine.GameState, mySeat int) int {
	if h == nil || gs == nil || mySeat < 0 || mySeat >= len(gs.Seats) {
		return -1
	}
	bestSeat := -1
	bestThreat := -1.0
	for i, s := range gs.Seats {
		if i == mySeat || s == nil || s.Lost || s.LeftGame {
			continue
		}
		t := 0.0
		if i < len(h.opponentProfiles) && h.opponentProfiles[i] != nil {
			t = h.opponentProfiles[i].ThreatLevel
		}
		if t > bestThreat {
			bestThreat = t
			bestSeat = i
		}
	}
	if bestThreat <= 0 {
		// Profile path didn't surface a meaningful threat — fall back to
		// raw offensive power. Useful in the early game / before classify
		// runs, and as a defensive default when profiles are stale.
		bestSeat = -1
		bestPow := -1.0
		for i, s := range gs.Seats {
			if i == mySeat || s == nil || s.Lost || s.LeftGame {
				continue
			}
			pow := effectiveOffensivePower(gs, s)
			if pow > bestPow {
				bestPow = pow
				bestSeat = i
			}
		}
		if bestPow <= 0 {
			// Even the fallback found nothing — return the first living
			// opponent so callers always get a usable index when any
			// opponent exists.
			for i, s := range gs.Seats {
				if i == mySeat || s == nil || s.Lost || s.LeftGame {
					continue
				}
				return i
			}
			return -1
		}
	}
	return bestSeat
}

// ChooseVoteAgainstThreat picks the option index that votes against
// our most-threatening opponent. optionSeats is a parallel slice to
// the engine's options []string — each entry is the SEAT that voting
// for that option would target / hurt (Council's Judgment: controller
// of the permanent up for exile; Custodi Lich: same; Coercive Portal:
// the seat-target of the "carnage" arm; etc.). -1 entries indicate
// options with no seat target (e.g. a "draw cards" arm of Coercive
// Portal that doesn't punish anyone).
//
// Decision:
//   1. Find the most-threatening opponent via MostThreateningOpponent.
//   2. If any option's optionSeats[i] matches that opponent, return i.
//   3. Otherwise prefer any option with optionSeats[i] != mySeat and
//      != -1 (an option that hurts SOMEONE other than us, even if not
//      the threat leader). Picks the first such.
//   4. If no opponent-targeting option exists, return 0 (the engine
//      requires a valid index; abstaining is a separate engine path).
//
// Per-card handlers that have richer option semantics can either call
// this directly with optionSeats or build their own option scorer.
//
// Returns 0 for empty options (defensive — engine callers should never
// pass an empty slice, but the helper survives it).
func (h *YggdrasilHat) ChooseVoteAgainstThreat(gs *gameengine.GameState, mySeat int, optionSeats []int) int {
	if len(optionSeats) == 0 {
		return 0
	}
	threat := h.MostThreateningOpponent(gs, mySeat)
	if threat >= 0 {
		for i, s := range optionSeats {
			if s == threat {
				return i
			}
		}
	}
	// No option targets the threat leader — pick the first option that
	// at least targets an opponent (not ourselves and not -1).
	for i, s := range optionSeats {
		if s != mySeat && s >= 0 {
			return i
		}
	}
	return 0
}
