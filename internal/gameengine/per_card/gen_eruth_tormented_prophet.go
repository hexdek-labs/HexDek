package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEruthTormentedProphet wires Eruth, Tormented Prophet.
//
// Oracle text:
//
//   If you would draw a card, exile the top two cards of your library instead. You may play those cards this turn.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerEruthTormentedProphet(r *Registry) {
	r.OnETB("Eruth, Tormented Prophet", eruthTormentedProphetStaticETB)
}

func eruthTormentedProphetStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eruth_tormented_prophet_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
