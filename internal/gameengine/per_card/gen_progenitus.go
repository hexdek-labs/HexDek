package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerProgenitus wires Progenitus.
//
// Oracle text:
//
//   Protection from everything
//   If Progenitus would be put into a graveyard from anywhere, reveal Progenitus and shuffle it into its owner's library instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerProgenitus(r *Registry) {
	r.OnETB("Progenitus", progenitusStaticETB)
}

func progenitusStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "progenitus_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
