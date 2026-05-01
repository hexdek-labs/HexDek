package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhylathWorldSculptor wires Phylath, World Sculptor.
//
// Oracle text:
//
//   When Phylath enters, create a 0/1 green Plant creature token for each basic land you control.
//   Landfall — Whenever a land you control enters, put four +1/+1 counters on target Plant you control.
//
// Auto-generated ETB handler.
func registerPhylathWorldSculptor(r *Registry) {
	r.OnETB("Phylath, World Sculptor", phylathWorldSculptorETB)
}

func phylathWorldSculptorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "phylath_world_sculptor_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "0/1 Token Token",
		Owner:         seat,
		BasePower:     0,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
