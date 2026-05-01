package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYurlokOfScorchThrash wires Yurlok of Scorch Thrash.
//
// Oracle text:
//
//   Vigilance
//   A player losing unspent mana causes that player to lose that much life.
//   {1}, {T}: Each player adds {B}{R}{G}.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerYurlokOfScorchThrash(r *Registry) {
	r.OnETB("Yurlok of Scorch Thrash", yurlokOfScorchThrashStaticETB)
}

func yurlokOfScorchThrashStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yurlok_of_scorch_thrash_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
