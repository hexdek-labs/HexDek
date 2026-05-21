package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheMasterOfKeys wires The Master of Keys.
//
// Oracle text:
//
//	Flying
//	When The Master of Keys enters, put X +1/+1 counters on it and mill
//	twice X cards.
//	Each enchantment card in your graveyard has escape. The escape cost
//	is equal to the card's mana cost plus exile three other cards from
//	your graveyard.
//
// Implementation:
//   - X is the value paid into the cast cost; the engine stamps it into
//     `gs.Flags["_master_of_keys_x_<seat>"]` (mirrors the Walking
//     Ballista pattern). ETB reads X, applies X +1/+1 counters, mills 2X.
//   - Defensive guard: the X-flag is treated as the cost-honoring sentinel
//     — if X is absent or zero, the ETB is a no-op (the cast was made
//     for X=0 or the cost wasn't routed through the cast pipeline).
//   - Enchantment-escape grant remains an AST static; partial breadcrumb.
func registerTheMasterOfKeys(r *Registry) {
	r.OnETB("The Master of Keys", theMasterOfKeysETB)
}

func theMasterOfKeysETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "the_master_of_keys_etb"
	if gs == nil || perm == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	xKey := "_master_of_keys_x_0"
	if seatIdx > 0 {
		xKey = "_master_of_keys_x_" + string('0'+rune(seatIdx))
	}
	x := gs.Flags[xKey]
	delete(gs.Flags, xKey)
	if x < 0 {
		x = 0
	}
	if x > 0 {
		perm.AddCounter("+1/+1", x)
	}
	seat := gs.Seats[seatIdx]
	milled := 0
	for i := 0; i < 2*x && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, seatIdx, "library", "graveyard", "master_of_keys_mill")
		milled++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"x":        x,
		"counters": x,
		"milled":   milled,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"enchantment_escape_grant_static_handled_by_ast_layer")
}
