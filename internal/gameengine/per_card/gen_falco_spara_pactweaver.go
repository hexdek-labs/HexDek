package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFalcoSparaPactweaver wires Falco Spara, Pactweaver.
//
// Oracle text:
//
//   Flying, trample
//   Falco Spara enters with a shield counter on it.
//   You may look at the top card of your library any time.
//   You may cast spells from the top of your library by removing a counter from a creature you control in addition to paying their other costs.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerFalcoSparaPactweaver(r *Registry) {
	r.OnETB("Falco Spara, Pactweaver", falcoSparaPactweaverStaticETB)
}

func falcoSparaPactweaverStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "falco_spara_pactweaver_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
