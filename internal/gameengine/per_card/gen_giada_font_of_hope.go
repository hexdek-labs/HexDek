package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGiadaFontOfHope wires Giada, Font of Hope.
//
// Oracle text:
//
//   Flying, vigilance
//   Each other Angel you control enters with an additional +1/+1 counter on it for each Angel you already control.
//   {T}: Add {W}. Spend this mana only to cast an Angel spell.
//
// Auto-generated activated ability handler.
func registerGiadaFontOfHope(r *Registry) {
	r.OnActivated("Giada, Font of Hope", giadaFontOfHopeActivate)
}

func giadaFontOfHopeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "giada_font_of_hope_activate"
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
