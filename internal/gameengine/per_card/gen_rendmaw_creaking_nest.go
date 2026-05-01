package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRendmawCreakingNest wires Rendmaw, Creaking Nest.
//
// Oracle text:
//
//   Menace, reach
//   When Rendmaw enters and whenever you play a card with two or more card types, each player creates a tapped 2/2 black Bird creature token with flying. The tokens are goaded for the rest of the game. (They attack each combat if able and attack a player other than you if able.)
//
// Auto-generated ETB handler.
func registerRendmawCreakingNest(r *Registry) {
	r.OnETB("Rendmaw, Creaking Nest", rendmawCreakingNestETB)
}

func rendmawCreakingNestETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "rendmaw_creaking_nest_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "2/2 Bird Token",
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"token", "creature", "bird"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
