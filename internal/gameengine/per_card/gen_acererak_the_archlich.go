package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAcererakTheArchlich wires Acererak the Archlich.
//
// Oracle text:
//
//   When Acererak enters, if you haven't completed Tomb of Annihilation, return Acererak to its owner's hand and venture into the dungeon.
//   Whenever Acererak attacks, for each opponent, you create a 2/2 black Zombie creature token unless that player sacrifices a creature of their choice.
//
// Auto-generated ETB handler.
func registerAcererakTheArchlich(r *Registry) {
	r.OnETB("Acererak the Archlich", acererakTheArchlichETB)
}

func acererakTheArchlichETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "acererak_the_archlich_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "2/2 Zombie Token",
		Owner:         seat,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"token", "creature", "zombie"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
