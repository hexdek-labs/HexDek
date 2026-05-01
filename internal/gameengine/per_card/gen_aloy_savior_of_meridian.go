package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAloySaviorOfMeridian wires Aloy, Savior of Meridian.
//
// Oracle text:
//
//   Vigilance, reach
//   In You, All Things Are Possible — Whenever one or more artifact creatures you control attack, discover X, where X is the greatest power among them. (Exile cards from the top of your library until you exile a nonland card with that mana value or less. Cast it without paying its mana cost or put it into your hand. Put the rest on the bottom in a random order.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAloySaviorOfMeridian(r *Registry) {
	r.OnETB("Aloy, Savior of Meridian", aloySaviorOfMeridianStaticETB)
}

func aloySaviorOfMeridianStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aloy_savior_of_meridian_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
