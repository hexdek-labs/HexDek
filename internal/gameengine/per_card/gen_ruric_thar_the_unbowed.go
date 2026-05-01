package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRuricTharTheUnbowed wires Ruric Thar, the Unbowed.
//
// Oracle text:
//
//   Vigilance, reach
//   Ruric Thar attacks each combat if able.
//   Whenever a player casts a noncreature spell, Ruric Thar deals 6 damage to that player.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRuricTharTheUnbowed(r *Registry) {
	r.OnETB("Ruric Thar, the Unbowed", ruricTharTheUnbowedStaticETB)
}

func ruricTharTheUnbowedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ruric_thar_the_unbowed_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
