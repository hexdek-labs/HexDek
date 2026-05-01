package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTorbranThaneOfRedFell wires Torbran, Thane of Red Fell.
//
// Oracle text:
//
//   If a red source you control would deal damage to an opponent or a permanent an opponent controls, it deals that much damage plus 2 instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTorbranThaneOfRedFell(r *Registry) {
	r.OnETB("Torbran, Thane of Red Fell", torbranThaneOfRedFellStaticETB)
}

func torbranThaneOfRedFellStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "torbran_thane_of_red_fell_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
