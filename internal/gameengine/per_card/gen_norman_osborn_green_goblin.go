package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNormanOsbornGreenGoblin wires Norman Osborn // Green Goblin.
//
// Oracle text:
//
//   Norman Osborn can't be blocked.
//   Whenever Norman Osborn deals combat damage to a player, he connives. (Draw a card, then discard a card. If you discarded a nonland card, put a +1/+1 counter on this creature.)
//   {1}{U}{B}{R}: Transform Norman Osborn. Activate only as a sorcery.
//   Flying, menace
//   Spells you cast from your graveyard cost {2} less to cast.
//   Goblin Formula — Each nonland card in your graveyard has mayhem. The mayhem cost is equal to its mana cost. (You may cast a card from your graveyard for its mayhem cost if you discarded it this turn. Timing rules still apply.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerNormanOsbornGreenGoblin(r *Registry) {
	r.OnETB("Norman Osborn // Green Goblin", normanOsbornGreenGoblinStaticETB)
}

func normanOsbornGreenGoblinStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "norman_osborn_green_goblin_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
