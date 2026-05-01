package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFeatherTheRedeemed wires Feather, the Redeemed.
//
// Oracle text:
//
//   Flying
//   Whenever you cast an instant or sorcery spell that targets a creature you control, exile that card instead of putting it into your graveyard as it resolves. If you do, return it to your hand at the beginning of the next end step.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerFeatherTheRedeemed(r *Registry) {
	r.OnETB("Feather, the Redeemed", featherTheRedeemedStaticETB)
}

func featherTheRedeemedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "feather_the_redeemed_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
