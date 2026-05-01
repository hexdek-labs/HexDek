package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheFirstSliver wires The First Sliver.
//
// Oracle text:
//
//   Cascade (When you cast this spell, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it without paying its mana cost. Put the exiled cards on the bottom in a random order.)
//   Sliver spells you cast have cascade.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheFirstSliver(r *Registry) {
	r.OnETB("The First Sliver", theFirstSliverStaticETB)
}

func theFirstSliverStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_first_sliver_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
