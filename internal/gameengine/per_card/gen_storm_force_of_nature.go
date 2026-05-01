package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerStormForceOfNature wires Storm, Force of Nature.
//
// Oracle text:
//
//   Flying, vigilance
//   Ceaseless Tempest — Whenever Storm deals combat damage to a player, the next instant or sorcery spell you cast this turn has storm. (When you cast it, copy it for each spell cast before it this turn. You may choose new targets for the copies.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerStormForceOfNature(r *Registry) {
	r.OnETB("Storm, Force of Nature", stormForceOfNatureStaticETB)
}

func stormForceOfNatureStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "storm_force_of_nature_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
