package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKataraTheFearless wires Katara, the Fearless.
//
// Oracle text:
//
//   If a triggered ability of an Ally you control triggers, that ability triggers an additional time.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKataraTheFearless(r *Registry) {
	r.OnETB("Katara, the Fearless", kataraTheFearlessStaticETB)
}

func kataraTheFearlessStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "katara_the_fearless_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
