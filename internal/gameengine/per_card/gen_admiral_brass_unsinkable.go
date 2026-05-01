package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAdmiralBrassUnsinkable wires Admiral Brass, Unsinkable.
//
// Oracle text:
//
//   When Admiral Brass enters, mill four cards.
//   At the beginning of combat on your turn, you may return target Pirate creature card from your graveyard to the battlefield with a finality counter on it. It has base power and toughness 4/4. It gains haste until end of turn. (If a creature with a finality counter on it would die, exile it instead.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAdmiralBrassUnsinkable(r *Registry) {
	r.OnETB("Admiral Brass, Unsinkable", admiralBrassUnsinkableStaticETB)
}

func admiralBrassUnsinkableStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "admiral_brass_unsinkable_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
