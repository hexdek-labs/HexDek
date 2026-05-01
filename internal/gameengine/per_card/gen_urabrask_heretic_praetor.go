package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrabraskHereticPraetor wires Urabrask, Heretic Praetor.
//
// Oracle text:
//
//   Haste
//   At the beginning of your upkeep, exile the top card of your library. You may play it this turn.
//   At the beginning of each opponent's upkeep, the next time they would draw a card this turn, instead they exile the top card of their library. They may play it this turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerUrabraskHereticPraetor(r *Registry) {
	r.OnETB("Urabrask, Heretic Praetor", urabraskHereticPraetorStaticETB)
}

func urabraskHereticPraetorStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "urabrask_heretic_praetor_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
