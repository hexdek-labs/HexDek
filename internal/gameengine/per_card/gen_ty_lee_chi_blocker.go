package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTyLeeChiBlocker wires Ty Lee, Chi Blocker.
//
// Oracle text:
//
//   Flash
//   Prowess (Whenever you cast a noncreature spell, this creature gets +1/+1 until end of turn.)
//   When Ty Lee enters, tap up to one target creature. It doesn't untap during its controller's untap step for as long as you control Ty Lee.
//
// Auto-generated ETB handler.
func registerTyLeeChiBlocker(r *Registry) {
	r.OnETB("Ty Lee, Chi Blocker", tyLeeChiBlockerETB)
}

func tyLeeChiBlockerETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "ty_lee_chi_blocker_etb"
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
