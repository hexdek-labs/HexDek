package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOonaQueenOfTheFae wires Oona, Queen of the Fae.
//
// Oracle text:
//
//   Flying
//   {X}{U/B}: Choose a color. Target opponent exiles the top X cards of their library. For each card of the chosen color exiled this way, create a 1/1 blue and black Faerie Rogue creature token with flying.
//
// Auto-generated activated ability handler.
func registerOonaQueenOfTheFae(r *Registry) {
	r.OnActivated("Oona, Queen of the Fae", oonaQueenOfTheFaeActivate)
}

func oonaQueenOfTheFaeActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "oona_queen_of_the_fae_activate"
	if gs == nil || src == nil {
		return
	}
	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Faerie Token",
		Owner:         src.Controller,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "faerie"},
	}
	enterBattlefieldWithETB(gs, src.Controller, token, false)
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
