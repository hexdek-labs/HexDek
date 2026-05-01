package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWilsonRefinedGrizzly wires Wilson, Refined Grizzly.
//
// Oracle text:
//
//   This spell can't be countered.
//   Vigilance, reach, trample
//   Ward {2} (Whenever this creature becomes the target of a spell or ability an opponent controls, counter it unless that player pays {2}.)
//   Choose a Background (You can have a Background as a second commander.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerWilsonRefinedGrizzly(r *Registry) {
	r.OnETB("Wilson, Refined Grizzly", wilsonRefinedGrizzlyStaticETB)
}

func wilsonRefinedGrizzlyStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wilson_refined_grizzly_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
