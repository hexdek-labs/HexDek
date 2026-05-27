package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDigThroughTime wires Dig Through Time.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Dig%20Through%20Time):
//
//	Delve (Each card you exile from your graveyard while casting this
//	spell pays for {1}.)
//	Look at the top seven cards of your library. Put two of them into
//	your hand and the rest on the bottom of your library in any order.
//
// {6}{U}{U} Instant. The premier blue dig — cast for {U}{U} after
// delving 6, see 7 deep, keep the 2 best. Delve cost is AST-pipeline
// territory; this handler implements the look-keep-bottom effect.
//
// Implementation:
//   - OnResolve: take top 7 (or fewer if library shorter), rank by
//     sylvanCardKeepScore (lands score 0, 1-CMC score 1, mid 2-3,
//     bombs 4) DESCENDING, take top 2 to hand, bottom the rest. The
//     "in any order" clause lets us put the most-useful remaining
//     card on the bottom (so it's the LAST drawn). For simplicity,
//     bottom order is descending — useful cards go deeper so the
//     near-bottom is "junk first" — this means the unkeepable cards
//     get re-drawn last.
func registerDigThroughTime(r *Registry) {
	r.OnResolve("Dig Through Time", digThroughTimeResolve)
}

func digThroughTimeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "dig_through_time"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	n := 7
	if n > len(s.Library) {
		n = len(s.Library)
	}
	if n == 0 {
		emitFail(gs, slug, "Dig Through Time", "empty_library", nil)
		return
	}

	top := make([]*gameengine.Card, n)
	copy(top, s.Library[:n])
	s.Library = s.Library[n:]

	// Rank by keep score descending.
	for i := 0; i < len(top)-1; i++ {
		for j := i + 1; j < len(top); j++ {
			if sylvanCardKeepScore(top[j]) > sylvanCardKeepScore(top[i]) {
				top[i], top[j] = top[j], top[i]
			}
		}
	}

	keepN := 2
	if keepN > len(top) {
		keepN = len(top)
	}
	kept := []string{}
	for i := 0; i < keepN; i++ {
		gameengine.MoveCard(gs, top[i], seat, "library", "hand", "dig_through_time_keep")
		kept = append(kept, top[i].DisplayName())
	}

	// Bottom the rest. The slice is already descending by keep score,
	// so cards at positions [keepN:] are the lowest-scoring of the 7.
	// Append in current order — the next library draw will be the
	// least-useful of the bottomed cards (those came from positions
	// keepN, keepN+1, ...). Reverse it so the WORST (lowest score)
	// goes to the very bottom — i.e., we'll draw the marginally-better
	// of the rejects next.
	bottomed := make([]*gameengine.Card, 0, len(top)-keepN)
	for i := keepN; i < len(top); i++ {
		bottomed = append(bottomed, top[i])
	}
	// Reverse so the worst is appended last (deepest).
	for i, j := 0, len(bottomed)-1; i < j; i, j = i+1, j-1 {
		bottomed[i], bottomed[j] = bottomed[j], bottomed[i]
	}
	s.Library = append(s.Library, bottomed...)

	emit(gs, slug, "Dig Through Time", map[string]interface{}{
		"seat":     seat,
		"looked":   n,
		"kept":     kept,
		"bottomed": len(bottomed),
	})
}
