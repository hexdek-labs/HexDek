package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaphaelNinjaDestroyer wires Raphael, Ninja Destroyer.
//
// Oracle text:
//
//   Raphael must be blocked if able.
//   Enrage — Whenever Raphael is dealt damage, add that much {R}. Until end of turn, you don't lose this mana as steps and phases end.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerRaphaelNinjaDestroyer(r *Registry) {
	r.OnETB("Raphael, Ninja Destroyer", raphaelNinjaDestroyerStaticETB)
}

func raphaelNinjaDestroyerStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "raphael_ninja_destroyer_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
