package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSliverGravemother wires Sliver Gravemother.
//
// Oracle text:
//
//   The "legend rule" doesn't apply to Slivers you control.
//   Each Sliver creature card in your graveyard has encore {X}, where X is its mana value.
//   Encore {5} ({5}, Exile this card from your graveyard: For each opponent, create a token copy that attacks that opponent this turn if able. They gain haste. Sacrifice them at the beginning of the next end step. Activate only as a sorcery.)
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerSliverGravemother(r *Registry) {
	r.OnETB("Sliver Gravemother", sliverGravemotherStaticETB)
}

func sliverGravemotherStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "sliver_gravemother_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
