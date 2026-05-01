package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZaffaiAndTheTempests wires Zaffai and the Tempests.
//
// Oracle text:
//
//   Once during each of your turns, you may cast an instant or sorcery spell from your hand without paying its mana cost.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerZaffaiAndTheTempests(r *Registry) {
	r.OnETB("Zaffai and the Tempests", zaffaiAndTheTempestsStaticETB)
}

func zaffaiAndTheTempestsStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zaffai_and_the_tempests_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
