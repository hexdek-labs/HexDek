package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMahaItsFeathersNight wires Maha, Its Feathers Night.
//
// Oracle text:
//
//   Flying, trample
//   Ward—Discard a card.
//   Creatures your opponents control have base toughness 1.
//
// Auto-generated static ability stub (partial — engine handles most statics via AST).
func registerMahaItsFeathersNight(r *Registry) {
	r.OnETB("Maha, Its Feathers Night", mahaItsFeathersNightStaticETB)
}

func mahaItsFeathersNightStaticETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "maha_its_feathers_night_static"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "static abilities handled by AST engine; per_card stub for registration tracking")
}
