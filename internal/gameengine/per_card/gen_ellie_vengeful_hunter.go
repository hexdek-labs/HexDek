package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEllieVengefulHunter wires Ellie, Vengeful Hunter.
//
// Oracle text:
//
//   Pay 2 life, Sacrifice another creature: Ellie deals 2 damage to target player and gains indestructible until end of turn.
//   Partner—Survivors (You can have two commanders if both have this ability.)
//
// Auto-generated activated ability handler.
func registerEllieVengefulHunter(r *Registry) {
	r.OnActivated("Ellie, Vengeful Hunter", ellieVengefulHunterActivate)
}

func ellieVengefulHunterActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "ellie_vengeful_hunter_activate"
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
