package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUrzaPrinceOfKroog wires Urza, Prince of Kroog.
//
// Oracle text:
//
//   Artifact creatures you control get +2/+2.
//   {6}: Create a token that's a copy of target artifact you control, except it's a 1/1 Soldier creature in addition to its other types.
//
// Auto-generated activated ability handler.
func registerUrzaPrinceOfKroog(r *Registry) {
	r.OnActivated("Urza, Prince of Kroog", urzaPrinceOfKroogActivate)
}

func urzaPrinceOfKroogActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "urza_prince_of_kroog_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Soldier Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "soldier"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
