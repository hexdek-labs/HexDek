package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCecilyHauntedMage wires Cecily, Haunted Mage.
//
// Oracle text:
//
//   Your maximum hand size is eleven.
//   Whenever Cecily, Haunted Mage attacks, you draw a card and you lose 1 life. Then if you have eleven or more cards in your hand, you may cast an instant or sorcery spell from your hand without paying its mana cost.
//   Partner—Friends forever (You can have two commanders if both have this ability.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerCecilyHauntedMage(r *Registry) {
	r.OnETB("Cecily, Haunted Mage", cecilyHauntedMageStaticETB)
}

func cecilyHauntedMageStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cecily_haunted_mage_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
