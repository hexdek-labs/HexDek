package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAshlingFlameDancer wires Ashling, Flame Dancer.
//
// Oracle text:
//
//   You don't lose unspent red mana as steps and phases end.
//   Magecraft — Whenever you cast or copy an instant or sorcery spell, discard a card, then draw a card. If this is the second time this ability has resolved this turn, Ashling deals 2 damage to each opponent and each creature they control. If it's the third time, add {R}{R}{R}{R}.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAshlingFlameDancer(r *Registry) {
	r.OnETB("Ashling, Flame Dancer", ashlingFlameDancerStaticETB)
}

func ashlingFlameDancerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ashling_flame_dancer_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
