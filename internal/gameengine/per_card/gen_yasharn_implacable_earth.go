package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYasharnImplacableEarth wires Yasharn, Implacable Earth.
//
// Oracle text:
//
//   When Yasharn enters, search your library for a basic Forest card and a basic Plains card, reveal those cards, put them into your hand, then shuffle.
//   Players can't pay life or sacrifice nonland permanents to cast spells or activate abilities.
//
// Auto-generated ETB handler.
func registerYasharnImplacableEarth(r *Registry) {
	r.OnETB("Yasharn, Implacable Earth", yasharnImplacableEarthETB)
}

func yasharnImplacableEarthETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "yasharn_implacable_earth_etb"
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
