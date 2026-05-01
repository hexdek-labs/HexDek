package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRakdosLordOfRiots wires Rakdos, Lord of Riots.
//
// Oracle text:
//
//   You can't cast Rakdos unless an opponent lost life this turn.
//   Flying, trample
//   Creature spells you cast cost {1} less to cast for each 1 life your opponents have lost this turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRakdosLordOfRiots(r *Registry) {
	r.OnETB("Rakdos, Lord of Riots", rakdosLordOfRiotsStaticETB)
}

func rakdosLordOfRiotsStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rakdos_lord_of_riots_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
