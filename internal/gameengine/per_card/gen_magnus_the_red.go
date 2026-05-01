package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMagnusTheRed wires Magnus the Red.
//
// Oracle text:
//
//   Flying
//   Unearthly Power — Instant and sorcery spells you cast cost {1} less to cast for each creature token you control.
//   Blade of Magnus — Whenever Magnus the Red deals combat damage to a player, create a 3/3 red Spawn creature token.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMagnusTheRed(r *Registry) {
	r.OnETB("Magnus the Red", magnusTheRedStaticETB)
}

func magnusTheRedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "magnus_the_red_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
