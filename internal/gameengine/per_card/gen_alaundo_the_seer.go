package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlaundoTheSeer wires Alaundo the Seer.
//
// Oracle text:
//
//   {T}: Draw a card, then exile a card from your hand and put a number of time counters on it equal to its mana value. It gains "When the last time counter is removed from this card, if it's exiled, you may cast it without paying its mana cost. If you cast a creature spell this way, it gains haste until end of turn." Then remove a time counter from each other card you own in exile.
//
// Auto-generated activated ability handler.
func registerAlaundoTheSeer(r *Registry) {
	r.OnActivated("Alaundo the Seer", alaundoTheSeerActivate)
}

func alaundoTheSeerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "alaundo_the_seer_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
