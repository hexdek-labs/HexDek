package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBreyaEtheriumShaper wires Breya, Etherium Shaper.
//
// Oracle text:
//
//   When Breya enters, create two 1/1 blue Thopter artifact creature tokens with flying.
//   {2}, Sacrifice two artifacts: Choose one —
//   • Breya deals 3 damage to target player or planeswalker.
//   • Target creature gets -4/-4 until end of turn.
//   • You gain 5 life.
//
// Auto-generated ETB handler.
func registerBreyaEtheriumShaper(r *Registry) {
	r.OnETB("Breya, Etherium Shaper", breyaEtheriumShaperETB)
}

func breyaEtheriumShaperETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "breya_etherium_shaper_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.GainLife(gs, seat, 5, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Thopter Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "thopter"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
