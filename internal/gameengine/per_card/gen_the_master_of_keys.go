package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterOfKeys wires The Master of Keys.
//
// Oracle text:
//
//   Flying
//   When The Master of Keys enters, put X +1/+1 counters on it and mill twice X cards.
//   Each enchantment card in your graveyard has escape. The escape cost is equal to the card's mana cost plus exile three other cards from your graveyard. (You may cast cards from your graveyard for their escape cost.)
//
// Auto-generated ETB handler.
func registerTheMasterOfKeys(r *Registry) {
	r.OnETB("The Master of Keys", theMasterOfKeysETB)
}

func theMasterOfKeysETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_of_keys_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emitPartial(gs, slug, perm.Card.DisplayName(), "additional non-ETB abilities not implemented")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
