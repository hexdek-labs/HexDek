package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWillScionOfPeace wires Will, Scion of Peace.
//
// Oracle text:
//
//   Vigilance
//   {T}: Spells you cast this turn that are white and/or blue cost {X} less to cast, where X is the amount of life you gained this turn. Activate only as a sorcery.
//
// Auto-generated activated ability handler.
func registerWillScionOfPeace(r *Registry) {
	r.OnActivated("Will, Scion of Peace", willScionOfPeaceActivate)
}

func willScionOfPeaceActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "will_scion_of_peace_activate"
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
