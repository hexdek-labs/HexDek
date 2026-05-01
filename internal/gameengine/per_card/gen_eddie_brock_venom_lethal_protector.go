package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEddieBrockVenomLethalProtector wires Eddie Brock // Venom, Lethal Protector.
//
// Oracle text:
//
//   When Eddie Brock enters, return target creature card with mana value 1 or less from your graveyard to the battlefield.
//   {3}{B}{R}{G}: Transform Eddie Brock. Activate only as a sorcery.
//   Menace, trample, haste
//   Whenever Venom attacks, you may sacrifice another creature. If you do, draw X cards, then you may put a permanent card with mana value X or less from your hand onto the battlefield, where X is the sacrificed creature's mana value.
//
// Auto-generated ETB handler.
func registerEddieBrockVenomLethalProtector(r *Registry) {
	r.OnETB("Eddie Brock // Venom, Lethal Protector", eddieBrockVenomLethalProtectorETB)
}

func eddieBrockVenomLethalProtectorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "eddie_brock_venom_lethal_protector_etb"
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
