package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDionusElvishArchdruid wires Dionus, Elvish Archdruid.
//
// Oracle text:
//
//   Elves you control have "Whenever this creature becomes tapped during your turn, untap it and put a +1/+1 counter on it. This ability triggers only once each turn."
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerDionusElvishArchdruid(r *Registry) {
	r.OnETB("Dionus, Elvish Archdruid", dionusElvishArchdruidStaticETB)
}

func dionusElvishArchdruidStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "dionus_elvish_archdruid_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
