package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHidetsuguAndKairi wires Hidetsugu and Kairi.
//
// Oracle text:
//
//   Flying
//   When Hidetsugu and Kairi enters, draw three cards, then put two cards from your hand on top of your library in any order.
//   When Hidetsugu and Kairi dies, exile the top card of your library. Target opponent loses life equal to its mana value. If it's an instant or sorcery card, you may cast it without paying its mana cost.
//
// Auto-generated ETB handler.
func registerHidetsuguAndKairi(r *Registry) {
	r.OnETB("Hidetsugu and Kairi", hidetsuguAndKairiETB)
}

func hidetsuguAndKairiETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hidetsugu_and_kairi_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	for i := 0; i < 3; i++ {
		drawOne(gs, seat, perm.Card.DisplayName())
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
