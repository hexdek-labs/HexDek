package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTeysaKarlov wires Teysa Karlov.
//
// Oracle text:
//
//   If a creature dying causes a triggered ability of a permanent you control to trigger, that ability triggers an additional time.
//   Creature tokens you control have vigilance and lifelink.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTeysaKarlov(r *Registry) {
	r.OnETB("Teysa Karlov", teysaKarlovStaticETB)
}

func teysaKarlovStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "teysa_karlov_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
