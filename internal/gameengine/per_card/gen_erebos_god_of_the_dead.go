package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerErebosGodOfTheDead wires Erebos, God of the Dead.
//
// Oracle text:
//
//   Indestructible
//   As long as your devotion to black is less than five, Erebos isn't a creature. (Each {B} in the mana costs of permanents you control counts toward your devotion to black.)
//   Your opponents can't gain life.
//   {1}{B}, Pay 2 life: Draw a card.
//
// Auto-generated activated ability handler.
func registerErebosGodOfTheDead(r *Registry) {
	r.OnActivated("Erebos, God of the Dead", erebosGodOfTheDeadActivate)
}

func erebosGodOfTheDeadActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "erebos_god_of_the_dead_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drawOne(gs, src.Controller, src.Card.DisplayName())
	gameengine.GainLife(gs, src.Controller, 1, src.Card.DisplayName())
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
