package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShadowheartDarkJusticiar wires Shadowheart, Dark Justiciar.
//
// Oracle text:
//
//   {1}{B}, {T}, Sacrifice another creature: Draw X cards, where X is that creature's power.
//   Choose a Background (You can have a Background as a second commander.)
//
// Auto-generated activated ability handler.
func registerShadowheartDarkJusticiar(r *Registry) {
	r.OnActivated("Shadowheart, Dark Justiciar", shadowheartDarkJusticiarActivate)
}

func shadowheartDarkJusticiarActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "shadowheart_dark_justiciar_activate"
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
