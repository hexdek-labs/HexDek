package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerArchelosLagoonMystic wires Archelos, Lagoon Mystic.
//
// Oracle text:
//
//   As long as Archelos is tapped, other permanents enter tapped.
//   As long as Archelos is untapped, other permanents enter untapped.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerArchelosLagoonMystic(r *Registry) {
	r.OnETB("Archelos, Lagoon Mystic", archelosLagoonMysticStaticETB)
}

func archelosLagoonMysticStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "archelos_lagoon_mystic_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
