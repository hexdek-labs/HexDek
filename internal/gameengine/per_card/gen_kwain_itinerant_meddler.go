package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKwainItinerantMeddler wires Kwain, Itinerant Meddler.
//
// Oracle text (Scryfall, verified):
//
//	{T}: Each player may draw a card, then each player who drew a card
//	this way gains 1 life.
//
// Implementation (R49 stub port):
//   - {T} cost: defensive Tapped check + set Tapped=true.
//   - "Each player may draw" — policy: all non-Lost seats opt in.
//     The Kwain controller benefits from the lifegain symmetry, and
//     a "skip if hand is full" filter would need a hand-size lookup
//     plus a planner — out of scope for the per_card layer. Mirrors
//     other group-hug card draw helpers (Howling Mine path).
//   - "Then each player who drew gains 1 life" — only seats whose
//     drawOne succeeded (returned a non-nil card) tick life. An empty
//     library yields no draw and no life, matching the printed
//     conjunction.
//   - Tap-cost gate respects already-tapped state; sumLost seats
//     (Lost / LeftGame) are skipped entirely.
func registerKwainItinerantMeddler(r *Registry) {
	r.OnActivated("Kwain, Itinerant Meddler", kwainItinerantMeddlerActivate)
}

func kwainItinerantMeddlerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kwain_itinerant_meddler_activate"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	if src.Tapped {
		emitFail(gs, slug, src.Card.DisplayName(), "already_tapped", nil)
		return
	}
	src.Tapped = true

	drewSeats := []int{}
	for i, s := range gs.Seats {
		if s == nil || s.Lost || s.LeftGame {
			continue
		}
		if drawOne(gs, i, src.Card.DisplayName()) != nil {
			drewSeats = append(drewSeats, i)
		}
	}
	for _, i := range drewSeats {
		gameengine.GainLife(gs, i, 1, src.Card.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      src.Controller,
		"drew_seats": drewSeats,
	})
}
