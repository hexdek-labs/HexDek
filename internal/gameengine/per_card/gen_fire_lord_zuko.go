package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFireLordZuko wires Fire Lord Zuko.
//
// Oracle text:
//
//   Firebending X, where X is Fire Lord Zuko's power. (Whenever this creature attacks, add X {R}. This mana lasts until end of combat.)
//   Whenever you cast a spell from exile and whenever a permanent you control enters from exile, put a +1/+1 counter on each creature you control.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerFireLordZuko(r *Registry) {
	r.OnETB("Fire Lord Zuko", fireLordZukoStaticETB)
}

func fireLordZukoStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "fire_lord_zuko_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
