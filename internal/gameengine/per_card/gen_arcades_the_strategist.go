package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArcadesTheStrategist wires Arcades, the Strategist.
//
// Oracle text:
//
//   Flying, vigilance
//   Whenever a creature you control with defender enters, draw a card.
//   Each creature you control with defender assigns combat damage equal to its toughness rather than its power and can attack as though it didn't have defender.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerArcadesTheStrategist(r *Registry) {
	r.OnETB("Arcades, the Strategist", arcadesTheStrategistStaticETB)
}

func arcadesTheStrategistStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "arcades_the_strategist_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
