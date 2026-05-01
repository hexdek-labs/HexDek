package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhenaxGodOfDeception wires Phenax, God of Deception.
//
// Oracle text:
//
//   Indestructible
//   As long as your devotion to blue and black is less than seven, Phenax isn't a creature.
//   Creatures you control have "{T}: Target player mills X cards, where X is this creature's toughness."
//
// Auto-generated activated ability handler.
func registerPhenaxGodOfDeception(r *Registry) {
	r.OnActivated("Phenax, God of Deception", phenaxGodOfDeceptionActivate)
}

func phenaxGodOfDeceptionActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "phenax_god_of_deception_activate"
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
