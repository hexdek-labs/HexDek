package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYasminKhan wires Yasmin Khan.
//
// Oracle text:
//
//   {T}: Exile the top card of your library. Until your next end step, you may play it.
//   Doctor's companion (You can have two commanders if the other is the Doctor.)
//
// Auto-generated activated ability handler.
func registerYasminKhan(r *Registry) {
	r.OnActivated("Yasmin Khan", yasminKhanActivate)
}

func yasminKhanActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "yasmin_khan_activate"
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
