package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIsshinTwoHeavensAsOne wires Isshin, Two Heavens as One.
//
// Oracle text:
//
//   If a creature attacking causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerIsshinTwoHeavensAsOne(r *Registry) {
	r.OnETB("Isshin, Two Heavens as One", isshinTwoHeavensAsOneStaticETB)
}

func isshinTwoHeavensAsOneStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "isshin_two_heavens_as_one_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
