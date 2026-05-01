package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHamzaGuardianOfArashin wires Hamza, Guardian of Arashin.
//
// Oracle text:
//
//   This spell costs {1} less to cast for each creature you control with a +1/+1 counter on it.
//   Creature spells you cast cost {1} less to cast for each creature you control with a +1/+1 counter on it.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerHamzaGuardianOfArashin(r *Registry) {
	r.OnETB("Hamza, Guardian of Arashin", hamzaGuardianOfArashinStaticETB)
}

func hamzaGuardianOfArashinStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hamza_guardian_of_arashin_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
