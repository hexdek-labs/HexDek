package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQueenMarchesa wires Queen Marchesa.
//
// Oracle text:
//
//   Deathtouch, haste
//   When Queen Marchesa enters, you become the monarch.
//   At the beginning of your upkeep, if an opponent is the monarch, create a 1/1 black Assassin creature token with deathtouch and haste.
//
// Auto-generated ETB handler.
func registerQueenMarchesa(r *Registry) {
	r.OnETB("Queen Marchesa", queenMarchesaETB)
}

func queenMarchesaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "queen_marchesa_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Token Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
