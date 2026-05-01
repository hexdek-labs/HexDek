package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZethiArcaneBlademaster wires Zethi, Arcane Blademaster.
//
// Oracle text:
//
//   Multikicker {W/U}
//   When Zethi, Arcane Blademaster enters, exile up to X target instant cards from your graveyard, where X is the number of times Zethi was kicked. Put a kick counter on each of them.
//   Whenever Zethi attacks, copy each exiled card you own with a kick counter on it. You may cast the copies.
//
// Auto-generated ETB handler.
func registerZethiArcaneBlademaster(r *Registry) {
	r.OnETB("Zethi, Arcane Blademaster", zethiArcaneBlademasterETB)
}

func zethiArcaneBlademasterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zethi_arcane_blademaster_etb"
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
