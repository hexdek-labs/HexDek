package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLaraCroftTombRaider wires Lara Croft, Tomb Raider.
//
// Oracle text:
//
//   First strike, reach
//   Whenever Lara Croft attacks, exile up to one target legendary artifact card or legendary land card from a graveyard and put a discovery counter on it. You may play a card from exile with a discovery counter on it this turn.
//   Raid — At end of combat on your turn, if you attacked this turn, create a Treasure token.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerLaraCroftTombRaider(r *Registry) {
	r.OnETB("Lara Croft, Tomb Raider", laraCroftTombRaiderStaticETB)
}

func laraCroftTombRaiderStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lara_croft_tomb_raider_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
