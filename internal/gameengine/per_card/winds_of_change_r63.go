package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// winds_of_change_r63.go — per_card handler for Winds of Change.
//
// Oracle text (Scryfall):
//
//	Each player shuffles the cards from their hand into their library,
//	then draws that many cards.
//
// {R} Sorcery. A symmetric red "soft wheel": every player launders their
// current hand back into their deck and redraws a hand of the SAME size —
// card-neutral per player, but it refreshes a stuck hand and disrupts
// opponents who were holding specific answers. The cross-player,
// per-player-counted "shuffle hand in, draw that many" shape has no
// structured AST node, so the generic dispatch logs the clause as an
// inert raw-text residual and NOTHING happens (DOA). This is the
// snowflake the per_card layer exists for.
//
// Implementation (OnResolve; per_card resolve hook SKIPS the stock AST
// dispatch, so this is the authoritative spell body):
//   - For each living player, in seat order: move every card from hand
//     into library (the canonical MoveCard zone mover, so any leave-hand
//     / enter-library hooks fire), shuffle that library, then draw the
//     SAME count back. Library size is unchanged net; the hand is a fresh
//     draw of equal size.
//   - count is captured BEFORE the hand empties, so an empty hand draws 0
//     (and a player who somehow can't refill draws as many as remain).
func init() {
	registerWindsOfChangeR63(Global())
	AddResetHook(registerWindsOfChangeR63)
}

func registerWindsOfChangeR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Winds of Change", windsOfChangeResolve)
}

func windsOfChangeResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "winds_of_change"
	if gs == nil || item == nil {
		return
	}
	total := 0
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		// "shuffles the cards from their hand into their library"
		count := len(s.Hand)
		for len(gs.Seats[i].Hand) > 0 {
			c := gs.Seats[i].Hand[0]
			gameengine.MoveCard(gs, c, i, "hand", "library", "winds_of_change")
		}
		if gs.Rng != nil {
			lib := gs.Seats[i].Library
			gs.Rng.Shuffle(len(lib), func(a, b int) { lib[a], lib[b] = lib[b], lib[a] })
			gs.LogEvent(gameengine.Event{Kind: "shuffle", Seat: i, Target: i, Source: "Winds of Change"})
		}
		// "then draws that many cards"
		drawn := 0
		for j := 0; j < count && len(gs.Seats[i].Library) > 0; j++ {
			top := gs.Seats[i].Library[0]
			gameengine.MoveCard(gs, top, i, "library", "hand", "draw")
			drawn++
		}
		total += drawn
	}
	emit(gs, slug, "Winds of Change", map[string]interface{}{
		"seat":        item.Controller,
		"total_drawn": total,
	})
}
