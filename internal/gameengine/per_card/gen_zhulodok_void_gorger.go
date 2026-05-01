package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZhulodokVoidGorger wires Zhulodok, Void Gorger.
//
// Oracle text:
//
//   Colorless spells you cast from your hand with mana value 7 or greater have "Cascade, cascade." (When you cast one, exile cards from the top of your library until you exile a nonland card that costs less. You may cast it without paying its mana cost. Put the exiled cards on the bottom in a random order. Then do it again.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerZhulodokVoidGorger(r *Registry) {
	r.OnETB("Zhulodok, Void Gorger", zhulodokVoidGorgerStaticETB)
}

func zhulodokVoidGorgerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zhulodok_void_gorger_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
