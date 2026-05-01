package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZidaneTantalusThief wires Zidane, Tantalus Thief.
//
// Oracle text:
//
//   When Zidane enters, gain control of target creature an opponent controls until end of turn. Untap it. It gains lifelink and haste until end of turn.
//   Whenever an opponent gains control of a permanent from you, you create a Treasure token.
//
// Auto-generated ETB handler.
func registerZidaneTantalusThief(r *Registry) {
	r.OnETB("Zidane, Tantalus Thief", zidaneTantalusThiefETB)
}

func zidaneTantalusThiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "zidane_tantalus_thief_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	gameengine.GainLife(gs, seat, 1, perm.Card.DisplayName())
	token := &gameengine.Card{
		Name:          "1/1 Creature Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "creature"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
