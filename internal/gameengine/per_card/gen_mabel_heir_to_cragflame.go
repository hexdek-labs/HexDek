package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMabelHeirToCragflame wires Mabel, Heir to Cragflame.
//
// Oracle text (BLB, {1}{R}{W}, 3/3):
//
//	Other Mice you control get +1/+1.
//	When Mabel enters, create Cragflame, a legendary colorless Equipment
//	artifact token with "Equipped creature gets +1/+1 and has vigilance,
//	trample, and haste" and equip {2}.
//
// Implementation:
//   - Static "Other Mice +1/+1" handled by AST.
//   - ETB creates Cragflame as a legendary Equipment artifact token.
//     The token's grant text isn't applied until it's actually equipped
//     (engine has no auto-equip planner); we record the token's static
//     payload via a handler-readable Types tag so a future equip planner
//     can recognize it. emitPartial flags the equip-AI gap.
func registerMabelHeirToCragflame(r *Registry) {
	r.OnETB("Mabel, Heir to Cragflame", mabelHeirToCragflameETB)
}

func mabelHeirToCragflameETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mabel_cragflame_equipment_token"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:  "Cragflame",
		Owner: seat,
		Types: []string{"token", "legendary", "artifact", "equipment", "cragflame_equipment_grant"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  seat,
		"token": "Cragflame",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"cragflame_equip_grant_not_auto_attached_engine_lacks_equip_planner")
}
