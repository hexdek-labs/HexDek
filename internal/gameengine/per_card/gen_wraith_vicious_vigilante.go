package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWraithViciousVigilante wires Wraith, Vicious Vigilante.
//
// Oracle text:
//
//   Double strike
//   Fear Gas — Wraith can't be blocked.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerWraithViciousVigilante(r *Registry) {
	r.OnETB("Wraith, Vicious Vigilante", wraithViciousVigilanteStaticETB)
}

func wraithViciousVigilanteStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "wraith_vicious_vigilante_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
