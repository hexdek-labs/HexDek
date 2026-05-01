package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerInfiniteGuidelineStation wires Infinite Guideline Station.
//
// Oracle text:
//
//   When Infinite Guideline Station enters, create a tapped 2/2 colorless Robot artifact creature token for each multicolored permanent you control.
//   Station (Tap another creature you control: Put charge counters equal to its power on this Spacecraft. Station only as a sorcery. It's an artifact creature at 12+.)
//   12+ | Flying
//   Whenever Infinite Guideline Station attacks, draw a card for each multicolored permanent you control.
//
// Auto-generated ETB handler.
func registerInfiniteGuidelineStation(r *Registry) {
	r.OnETB("Infinite Guideline Station", infiniteGuidelineStationETB)
}

func infiniteGuidelineStationETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "infinite_guideline_station_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "2/2 Token Token",
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
