package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTifaLockhart wires Tifa Lockhart.
//
// Oracle text:
//
//   Trample
//   Landfall — Whenever a land you control enters, double Tifa Lockhart's power until end of turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTifaLockhart(r *Registry) {
	r.OnETB("Tifa Lockhart", tifaLockhartStaticETB)
}

func tifaLockhartStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "tifa_lockhart_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
