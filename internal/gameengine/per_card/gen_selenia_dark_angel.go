package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSeleniaDarkAngel wires Selenia, Dark Angel.
//
// Oracle text:
//
//   Flying
//   Pay 2 life: Return Selenia to its owner's hand.
//
// Auto-generated activated ability handler.
func registerSeleniaDarkAngel(r *Registry) {
	r.OnActivated("Selenia, Dark Angel", seleniaDarkAngelActivate)
}

func seleniaDarkAngelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "selenia_dark_angel_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
