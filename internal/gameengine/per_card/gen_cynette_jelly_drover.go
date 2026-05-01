package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCynetteJellyDrover wires Cynette, Jelly Drover.
//
// Oracle text:
//
//   When Cynette enters or dies, create a 1/1 blue Jellyfish creature token with flying.
//   Creatures you control with flying get +1/+1.
//
// Auto-generated ETB handler.
func registerCynetteJellyDrover(r *Registry) {
	r.OnETB("Cynette, Jelly Drover", cynetteJellyDroverETB)
}

func cynetteJellyDroverETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "cynette_jelly_drover_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Fish Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "fish"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
