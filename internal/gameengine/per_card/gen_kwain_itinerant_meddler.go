package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKwainItinerantMeddler wires Kwain, Itinerant Meddler.
//
// Oracle text:
//
//   {T}: Each player may draw a card, then each player who drew a card this way gains 1 life.
//
// Auto-generated activated ability handler.
func registerKwainItinerantMeddler(r *Registry) {
	r.OnActivated("Kwain, Itinerant Meddler", kwainItinerantMeddlerActivate)
}

func kwainItinerantMeddlerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "kwain_itinerant_meddler_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	gameengine.GainLife(gs, src.Controller, 1, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
