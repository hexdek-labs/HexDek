package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGhenArcanumWeaver wires Ghen, Arcanum Weaver.
//
// Oracle text:
//
//   {R}{W}{B}, {T}, Sacrifice an enchantment: Return target enchantment card from your graveyard to the battlefield.
//
// Auto-generated activated ability handler.
func registerGhenArcanumWeaver(r *Registry) {
	r.OnActivated("Ghen, Arcanum Weaver", ghenArcanumWeaverActivate)
}

func ghenArcanumWeaverActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ghen_arcanum_weaver_activate"
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
