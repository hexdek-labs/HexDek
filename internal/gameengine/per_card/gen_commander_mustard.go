package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCommanderMustard wires Commander Mustard.
//
// Oracle text:
//
//   Vigilance
//   Other Soldiers you control have vigilance, trample, and haste.
//   {2}{R}{W}: Until end of turn, Soldiers you control gain "Whenever this creature attacks, it deals 1 damage to defending player."
//
// Auto-generated activated ability handler.
func registerCommanderMustard(r *Registry) {
	r.OnActivated("Commander Mustard", commanderMustardActivate)
}

func commanderMustardActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "commander_mustard_activate"
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
