package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKruphixGodOfHorizons wires Kruphix, God of Horizons.
//
// Oracle text:
//
//   Indestructible
//   As long as your devotion to green and blue is less than seven, Kruphix isn't a creature.
//   You have no maximum hand size.
//   If you would lose unspent mana, that mana becomes colorless instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKruphixGodOfHorizons(r *Registry) {
	r.OnETB("Kruphix, God of Horizons", kruphixGodOfHorizonsStaticETB)
}

func kruphixGodOfHorizonsStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kruphix_god_of_horizons_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
