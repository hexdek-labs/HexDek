package hexapi

import (
	"fmt"
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// game_census_r131.go — make a heap abort describe the game it killed.
//
// r130 spent a round working out which decks were involved in the 18
// heap aborts, and could only answer it for 2 of them. The method was a
// join: an aborted game still emits a SLOW GAME line whose turns= equals
// the abort turn, and that line carries the deck list. The other 16
// games finished under the slow threshold, so they emit no such line and
// are permanently anonymous.
//
// Everything that join recovered — and much more — is in scope at the
// abort site in runOneGameFast: deckKeys, gameSeed, gs. The abort just
// never said any of it. It logged a turn number and a delta:
//
//	grinder: aborting game at turn 60 — heap delta 2553MB exceeds budget
//
// The event is roughly 1 in 30,000 games, so reproducing it on demand is
// not a realistic plan: two attempts across 425 games caught nothing,
// which is what the arithmetic predicts. The practical route is to make
// the NEXT occurrence explain itself.
//
// This matters more than it looks, because the seed is NOT a sufficient
// repro recipe and the judge's own violation text ("repro and
// sample-profile this seed") promises something the system cannot
// deliver: the hat's Curse DNA is drawn from a mutable, evolving pool
// using a different RNG than the game seed, so replaying a seed gives a
// differently-played game. Evidence has to be captured at the moment of
// failure or it is gone.

// gameCensus renders what a game actually CONTAINED at the moment a
// guard fired — the shape of the state, which is the thing an allocation
// explosion should show up in.
//
// Deliberately counts objects rather than sampling memory: a runaway is
// expected to show as an absurd number of SOMETHING (permanents, tokens,
// stack depth, graveyard), and which one it is points at the mechanic.
// Reads only exported fields and allocates one string.
func gameCensus(gs *gameengine.GameState) string {
	if gs == nil {
		return "census=nil"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "stack=%d seats=%d", len(gs.Stack), len(gs.Seats))
	totalPerms, totalTokens := 0, 0
	for i, seat := range gs.Seats {
		if seat == nil {
			fmt.Fprintf(&b, " s%d=nil", i)
			continue
		}
		tokens := 0
		for _, p := range seat.Battlefield {
			if p != nil && p.IsToken() {
				tokens++
			}
		}
		totalPerms += len(seat.Battlefield)
		totalTokens += tokens
		fmt.Fprintf(&b, " s%d=[life=%d bf=%d(tok=%d) hand=%d gy=%d exile=%d lib=%d]",
			i, seat.Life, len(seat.Battlefield), tokens,
			len(seat.Hand), len(seat.Graveyard), len(seat.Exile), len(seat.Library))
	}
	fmt.Fprintf(&b, " perms=%d tokens=%d", totalPerms, totalTokens)
	return b.String()
}

// abortContext is the identity half: which game this was, so a future
// abort can be traced to its decks without the SLOW GAME join that only
// worked for 2 of 18.
func abortContext(seed int64, deckKeys []string) string {
	return fmt.Sprintf("seed=%d decks=[%s]", seed, strings.Join(deckKeys, " "))
}
