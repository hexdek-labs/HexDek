package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLierDiscipleOfTheDrowned wires Lier, Disciple of the Drowned.
//
// Oracle text:
//
//   Spells can't be countered.
//   Each instant and sorcery card in your graveyard has flashback. The flashback cost is equal to that card's mana cost.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerLierDiscipleOfTheDrowned(r *Registry) {
	r.OnETB("Lier, Disciple of the Drowned", lierDiscipleOfTheDrownedStaticETB)
}

func lierDiscipleOfTheDrownedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lier_disciple_of_the_drowned_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
