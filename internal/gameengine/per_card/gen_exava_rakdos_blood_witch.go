package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerExavaRakdosBloodWitch wires Exava, Rakdos Blood Witch.
//
// Oracle text:
//
//   First strike, haste
//   Unleash (You may have this creature enter with a +1/+1 counter on it. It can't block as long as it has a +1/+1 counter on it.)
//   Each other creature you control with a +1/+1 counter on it has haste.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerExavaRakdosBloodWitch(r *Registry) {
	r.OnETB("Exava, Rakdos Blood Witch", exavaRakdosBloodWitchStaticETB)
}

func exavaRakdosBloodWitchStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "exava_rakdos_blood_witch_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
