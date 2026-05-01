package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAzamiLadyOfScrolls wires Azami, Lady of Scrolls.
//
// Oracle text:
//
//   Tap an untapped Wizard you control: Draw a card.
//
// Auto-generated activated ability handler.
func registerAzamiLadyOfScrolls(r *Registry) {
	r.OnActivated("Azami, Lady of Scrolls", azamiLadyOfScrollsActivate)
}

func azamiLadyOfScrollsActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "azami_lady_of_scrolls_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
