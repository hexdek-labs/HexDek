package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDereviEmpyrialTactician wires Derevi, Empyrial Tactician.
//
// Oracle text:
//
//   Flying
//   When Derevi enters and whenever a creature you control deals combat damage to a player, you may tap or untap target permanent.
//   {1}{G}{W}{U}: Put Derevi onto the battlefield from the command zone.
//
// Auto-generated ETB handler.
func registerDereviEmpyrialTactician(r *Registry) {
	r.OnETB("Derevi, Empyrial Tactician", dereviEmpyrialTacticianETB)
}

func dereviEmpyrialTacticianETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "derevi_empyrial_tactician_etb"
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
