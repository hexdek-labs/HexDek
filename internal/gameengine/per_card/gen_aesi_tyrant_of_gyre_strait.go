package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAesiTyrantOfGyreStrait wires Aesi, Tyrant of Gyre Strait.
//
// Oracle text:
//
//   You may play an additional land on each of your turns.
//   Landfall — Whenever a land you control enters, you may draw a card.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerAesiTyrantOfGyreStrait(r *Registry) {
	r.OnETB("Aesi, Tyrant of Gyre Strait", aesiTyrantOfGyreStraitStaticETB)
}

func aesiTyrantOfGyreStraitStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "aesi_tyrant_of_gyre_strait_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
