package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCaptainAmericaFirstAvenger wires Captain America, First Avenger.
//
// Oracle text:
//
//   Throw ... — {3}, Unattach an Equipment from Captain America: He deals damage equal to that Equipment's mana value divided as you choose among one, two, or three targets.
//   ... Catch — At the beginning of combat on your turn, attach up to one target Equipment you control to Captain America.
//
// Auto-generated activated ability handler.
func registerCaptainAmericaFirstAvenger(r *Registry) {
	r.OnActivated("Captain America, First Avenger", captainAmericaFirstAvengerActivate)
}

func captainAmericaFirstAvengerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "captain_america_first_avenger_activate"
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
