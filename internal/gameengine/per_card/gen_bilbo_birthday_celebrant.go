package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBilboBirthdayCelebrant wires Bilbo, Birthday Celebrant.
//
// Oracle text:
//
//   If you would gain life, you gain that much life plus 1 instead.
//   {2}{W}{B}{G}, {T}, Exile Bilbo: Search your library for any number of creature cards, put them onto the battlefield, then shuffle. Activate only if you have 111 or more life.
//
// Auto-generated activated ability handler.
func registerBilboBirthdayCelebrant(r *Registry) {
	r.OnActivated("Bilbo, Birthday Celebrant", bilboBirthdayCelebrantActivate)
}

func bilboBirthdayCelebrantActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "bilbo_birthday_celebrant_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.GainLife(gs, src.Controller, 1, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
