package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRatKingVerminister wires Rat King, Verminister.
//
// Oracle text:
//
//   Disappear — At the beginning of your end step, if a permanent left the battlefield under your control this turn, create a 1/1 black Rat creature token and put a +1/+1 counter on Rat King.
//   {T}, Sacrifice three Rats: Return target creature card and all other cards with the same name as that card from your graveyard to the battlefield tapped.
//
// Auto-generated activated ability handler.
func registerRatKingVerminister(r *Registry) {
	r.OnActivated("Rat King, Verminister", ratKingVerministerActivate)
}

func ratKingVerministerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "rat_king_verminister_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Rat Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "rat"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	src.AddCounter("+1/+1", 1)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
