package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheWanderingMinstrel wires The Wandering Minstrel.
//
// Oracle text:
//
//   Lands you control enter untapped.
//   The Minstrel's Ballad — At the beginning of combat on your turn, if you control five or more Towns, create a 2/2 Elemental creature token that's all colors.
//   {3}{W}{U}{B}{R}{G}: Other creatures you control get +X/+X until end of turn, where X is the number of Towns you control.
//
// Auto-generated activated ability handler.
func registerTheWanderingMinstrel(r *Registry) {
	r.OnActivated("The Wandering Minstrel", theWanderingMinstrelActivate)
}

func theWanderingMinstrelActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "the_wandering_minstrel_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "2/2 Elemental Token",
		Owner:         src.Controller,
		BasePower:     2,
		BaseToughness: 2,
		Types:         []string{"token", "creature", "elemental"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
