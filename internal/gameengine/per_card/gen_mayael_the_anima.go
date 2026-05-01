package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMayaelTheAnima wires Mayael the Anima.
//
// Oracle text:
//
//   {3}{R}{G}{W}, {T}: Look at the top five cards of your library. You may put a creature card with power 5 or greater from among them onto the battlefield. Put the rest on the bottom of your library in any order.
//
// Auto-generated activated ability handler.
func registerMayaelTheAnima(r *Registry) {
	r.OnActivated("Mayael the Anima", mayaelTheAnimaActivate)
}

func mayaelTheAnimaActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "mayael_the_anima_activate"
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
