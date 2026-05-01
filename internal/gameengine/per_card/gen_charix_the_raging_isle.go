package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCharixTheRagingIsle wires Charix, the Raging Isle.
//
// Oracle text:
//
//   Spells your opponents cast that target Charix cost {2} more to cast.
//   {3}: Charix gets +X/-X until end of turn, where X is the number of Islands you control.
//
// Auto-generated activated ability handler.
func registerCharixTheRagingIsle(r *Registry) {
	r.OnActivated("Charix, the Raging Isle", charixTheRagingIsleActivate)
}

func charixTheRagingIsleActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "charix_the_raging_isle_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(), "auto-gen: activated effect not parsed from oracle text")
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
