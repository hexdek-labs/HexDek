package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKudoKingAmongBears wires Kudo, King Among Bears.
//
// Oracle text:
//
//   Other creatures have base power and toughness 2/2 and are Bears in addition to their other types.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKudoKingAmongBears(r *Registry) {
	r.OnETB("Kudo, King Among Bears", kudoKingAmongBearsStaticETB)
}

func kudoKingAmongBearsStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kudo_king_among_bears_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
