package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheEarthKing wires The Earth King.
//
// Oracle text:
//
//   When The Earth King enters, create a 4/4 green Bear creature token.
//   Whenever one or more creatures you control with power 4 or greater attack, search your library for up to that many basic land cards, put them onto the battlefield tapped, then shuffle.
//
// Auto-generated ETB handler.
func registerTheEarthKing(r *Registry) {
	r.OnETB("The Earth King", theEarthKingETB)
}

func theEarthKingETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_earth_king_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "4/4 Token Token",
		Owner:         seat,
		BasePower:     4,
		BaseToughness: 4,
		Types:         []string{"token", "creature", "token"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
