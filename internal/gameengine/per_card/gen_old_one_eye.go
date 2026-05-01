package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOldOneEye wires Old One Eye.
//
// Oracle text:
//
//   Trample
//   Other creatures you control have trample.
//   When Old One Eye enters, create a 5/5 green Tyranid creature token.
//   Fast Healing — At the beginning of your first main phase, you may discard two cards. If you do, return this card from your graveyard to your hand.
//
// Auto-generated ETB handler.
func registerOldOneEye(r *Registry) {
	r.OnETB("Old One Eye", oldOneEyeETB)
}

func oldOneEyeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "old_one_eye_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
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
