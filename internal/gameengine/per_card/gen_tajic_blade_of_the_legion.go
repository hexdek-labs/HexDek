package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTajicBladeOfTheLegion wires Tajic, Blade of the Legion.
//
// Oracle text:
//
//   Indestructible
//   Battalion — Whenever Tajic and at least two other creatures attack, Tajic gets +5/+5 until end of turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTajicBladeOfTheLegion(r *Registry) {
	r.OnETB("Tajic, Blade of the Legion", tajicBladeOfTheLegionStaticETB)
}

func tajicBladeOfTheLegionStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tajic_blade_of_the_legion_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
