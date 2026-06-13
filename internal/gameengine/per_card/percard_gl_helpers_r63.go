package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// percard_gl_helpers_r63.go — small shared helpers for the G–L DOA
// coverage batch (r63). New file; touches no shared registry/init.

// highestLifeOpponent returns the living opponent of `seat` with the most
// life, or -1 if none. The canonical "target player" pick when a spell's
// player target wasn't stamped onto the StackItem — matches the policy in
// inspired_ultimatum_r60 (hit the healthiest opponent).
func highestLifeOpponent(gs *gameengine.GameState, seat int) int {
	if gs == nil {
		return -1
	}
	best, bestLife := -1, -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if os.Life > bestLife {
			bestLife = os.Life
			best = opp
		}
	}
	return best
}
