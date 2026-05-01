package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSelvala wires Selvala, Explorer Returned.
//
// Oracle text:
//
//	Parley — {T}: Each player reveals the top card of their library.
//	For each nonland card revealed this way, add {G} and you gain 1 life.
//	Then each player draws a card.
//
// Activated ability index 0 — the parley tap. The handler peeks each
// seat's top library card without removing it, counts nonland reveals,
// adds that many {G} to the controller's pool, gains that much life,
// then walks every seat in turn order and draws one card.
func registerSelvala(r *Registry) {
	r.OnActivated("Selvala, Explorer Returned", selvalaParley)
}

func selvalaParley(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "selvala_parley"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	nonlandCount := 0
	revealed := make([]string, 0, len(gs.Seats))
	for i, s := range gs.Seats {
		if s == nil || s.Lost || len(s.Library) == 0 {
			revealed = append(revealed, "")
			continue
		}
		top := s.Library[0]
		name := ""
		if top != nil {
			name = top.DisplayName()
			if !cardHasType(top, "land") {
				nonlandCount++
			}
		}
		revealed = append(revealed, name)
		gs.LogEvent(gameengine.Event{
			Kind:   "reveal_top_of_library",
			Seat:   i,
			Source: src.Card.DisplayName(),
			Details: map[string]interface{}{
				"card":   name,
				"reason": "selvala_parley",
			},
		})
	}

	if nonlandCount > 0 {
		gameengine.AddMana(gs, gs.Seats[seat], "G", nonlandCount, src.Card.DisplayName())
		gameengine.GainLife(gs, seat, nonlandCount, src.Card.DisplayName())
	}

	// Each player draws a card, in turn order starting from the active
	// player (APNAP — close enough for "then each player draws").
	for _, idx := range gameengine.APNAPOrder(gs) {
		s := gs.Seats[idx]
		if s == nil || s.Lost || len(s.Library) == 0 {
			continue
		}
		card := s.Library[0]
		gameengine.MoveCard(gs, card, idx, "library", "hand", "draw")
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"nonland_count": nonlandCount,
		"green_added":   nonlandCount,
		"life_gained":   nonlandCount,
		"revealed":      revealed,
	})
}
