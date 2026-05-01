package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerJasmineBorealOfTheSeven wires Jasmine Boreal of the Seven.
//
// Oracle text:
//
//   {T}: Add {G}{W}. Spend this mana only to cast creature spells with no abilities.
//   Creatures you control with no abilities can't be blocked by creatures with abilities.
//
// Auto-generated activated ability handler.
func registerJasmineBorealOfTheSeven(r *Registry) {
	r.OnActivated("Jasmine Boreal of the Seven", jasmineBorealOfTheSevenActivate)
}

func jasmineBorealOfTheSevenActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "jasmine_boreal_of_the_seven_activate"
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
