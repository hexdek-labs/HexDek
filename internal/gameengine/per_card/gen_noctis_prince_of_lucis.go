package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNoctisPrinceOfLucis wires Noctis, Prince of Lucis.
//
// Oracle text:
//
//   Lifelink
//   You may cast artifact spells from your graveyard by paying 3 life in addition to paying their other costs. If you cast a spell this way, that artifact enters with a finality counter on it.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerNoctisPrinceOfLucis(r *Registry) {
	r.OnETB("Noctis, Prince of Lucis", noctisPrinceOfLucisStaticETB)
}

func noctisPrinceOfLucisStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "noctis_prince_of_lucis_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
