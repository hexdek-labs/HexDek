package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophTheFirstMetalbender wires Toph, the First Metalbender.
//
// Oracle text:
//
//   Nontoken artifacts you control are lands in addition to their other types. (They don't gain the ability to {T} for mana.)
//   At the beginning of your end step, earthbend 2. (Target land you control becomes a 0/0 creature with haste that's still a land. Put two +1/+1 counters on it. When it dies or is exiled, return it to the battlefield tapped.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerTophTheFirstMetalbender(r *Registry) {
	r.OnETB("Toph, the First Metalbender", tophTheFirstMetalbenderStaticETB)
}

func tophTheFirstMetalbenderStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_the_first_metalbender_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
