package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMabelHeirToCragflame wires Mabel, Heir to Cragflame.
//
// Oracle text:
//
//   Other Mice you control get +1/+1.
//   When Mabel enters, create Cragflame, a legendary colorless Equipment artifact token with "Equipped creature gets +1/+1 and has vigilance, trample, and haste" and equip {2}.
//
// Auto-generated ETB handler.
func registerMabelHeirToCragflame(r *Registry) {
	r.OnETB("Mabel, Heir to Cragflame", mabelHeirToCragflameETB)
}

func mabelHeirToCragflameETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mabel_heir_to_cragflame_etb"
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
