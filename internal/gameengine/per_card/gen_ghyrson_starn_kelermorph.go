package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGhyrsonStarnKelermorph wires Ghyrson Starn, Kelermorph.
//
// Oracle text:
//
//   Ward {2} (Whenever this creature becomes the target of a spell or ability an opponent controls, counter it unless that player pays {2}.)
//   Three Autostubs — Whenever another source you control deals exactly 1 damage to a permanent or player, Ghyrson Starn deals 2 damage to that permanent or player.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerGhyrsonStarnKelermorph(r *Registry) {
	r.OnETB("Ghyrson Starn, Kelermorph", ghyrsonStarnKelermorphStaticETB)
}

func ghyrsonStarnKelermorphStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ghyrson_starn_kelermorph_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
