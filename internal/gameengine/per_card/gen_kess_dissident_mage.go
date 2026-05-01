package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKessDissidentMage wires Kess, Dissident Mage.
//
// Oracle text:
//
//   Flying
//   Once during each of your turns, you may cast an instant or sorcery spell from your graveyard. If a spell cast this way would be put into your graveyard, exile it instead.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerKessDissidentMage(r *Registry) {
	r.OnETB("Kess, Dissident Mage", kessDissidentMageStaticETB)
}

func kessDissidentMageStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kess_dissident_mage_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
