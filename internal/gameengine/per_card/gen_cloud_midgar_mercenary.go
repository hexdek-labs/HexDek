package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCloudMidgarMercenary wires Cloud, Midgar Mercenary.
//
// Oracle text:
//
//   When Cloud enters, search your library for an Equipment card, reveal it, put it into your hand, then shuffle.
//   As long as Cloud is equipped, if a triggered ability of Cloud or an Equipment attached to it triggers, that ability triggers an additional time.
//
// Auto-generated ETB handler.
func registerCloudMidgarMercenary(r *Registry) {
	r.OnETB("Cloud, Midgar Mercenary", cloudMidgarMercenaryETB)
}

func cloudMidgarMercenaryETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cloud_midgar_mercenary_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
