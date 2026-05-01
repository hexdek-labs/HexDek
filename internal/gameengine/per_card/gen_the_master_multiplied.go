package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterMultiplied wires The Master, Multiplied.
//
// Oracle text:
//
//   Myriad
//   The "legend rule" doesn't apply to creature tokens you control.
//   Triggered abilities you control can't cause you to sacrifice or exile creature tokens you control.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTheMasterMultiplied(r *Registry) {
	r.OnETB("The Master, Multiplied", theMasterMultipliedStaticETB)
}

func theMasterMultipliedStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_multiplied_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
