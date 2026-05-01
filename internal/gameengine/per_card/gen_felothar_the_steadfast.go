package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFelotharTheSteadfast wires Felothar the Steadfast.
//
// Oracle text:
//
//   Each creature you control assigns combat damage equal to its toughness rather than its power.
//   Creatures you control can attack as though they didn't have defender.
//   {3}, {T}, Sacrifice another creature: Draw cards equal to the sacrificed creature's toughness, then discard cards equal to its power.
//
// Auto-generated activated ability handler.
func registerFelotharTheSteadfast(r *Registry) {
	r.OnActivated("Felothar the Steadfast", felotharTheSteadfastActivate)
}

func felotharTheSteadfastActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "felothar_the_steadfast_activate"
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
