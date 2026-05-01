package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheSwarmweaver wires The Swarmweaver.
//
// Oracle text:
//
//   When The Swarmweaver enters, create two 1/1 black and green Insect creature tokens with flying.
//   Delirium — As long as there are four or more card types among cards in your graveyard, Insects and Spiders you control get +1/+1 and have deathtouch.
//
// Auto-generated ETB handler.
func registerTheSwarmweaver(r *Registry) {
	r.OnETB("The Swarmweaver", theSwarmweaverETB)
}

func theSwarmweaverETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_swarmweaver_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	token := &gameengine.Card{
		Name:          "1/1 Insect Token",
		Owner:         seat,
		BasePower:     1,
		BaseToughness: 1,
		Types:         []string{"token", "creature", "insect"},
	}
	enterBattlefieldWithETB(gs, seat, token, false)
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
