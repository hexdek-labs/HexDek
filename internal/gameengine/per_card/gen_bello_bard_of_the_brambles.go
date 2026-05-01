package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBelloBardOfTheBrambles wires Bello, Bard of the Brambles.
//
// Oracle text:
//
//   During your turn, each non-Equipment artifact and non-Aura enchantment you control with mana value 4 or greater is a 4/4 Elemental creature in addition to its other types and has indestructible, haste, and "Whenever this creature deals combat damage to a player, draw a card."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerBelloBardOfTheBrambles(r *Registry) {
	r.OnETB("Bello, Bard of the Brambles", belloBardOfTheBramblesStaticETB)
}

func belloBardOfTheBramblesStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "bello_bard_of_the_brambles_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
