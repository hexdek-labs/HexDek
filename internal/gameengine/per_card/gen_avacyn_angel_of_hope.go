package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAvacynAngelOfHope wires Avacyn, Angel of Hope.
//
// Oracle text:
//
//   Flying, vigilance, indestructible
//   Other permanents you control have indestructible.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAvacynAngelOfHope(r *Registry) {
	r.OnETB("Avacyn, Angel of Hope", avacynAngelOfHopeStaticETB)
}

func avacynAngelOfHopeStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "avacyn_angel_of_hope_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
