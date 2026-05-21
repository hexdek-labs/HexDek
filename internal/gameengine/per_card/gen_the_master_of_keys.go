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
//   - Enchantment-escape grant (R58): register a ZoneCastPolicy that
//     lets the controller cast enchantment cards from their own
//     graveyard with ExileOnResolve set (so the cast routes the card to
//     exile per escape's "exile rather than graveyard" rider). The
//     printed additional cost — "plus exile three other cards from your
//     graveyard" — isn't representable in the policy primitive's
//     single-integer ManaCost slot; the mana-cost half is honored
//     (ManaCost=-1 uses the card's printed mana cost) and the additional
//     exile cost is approximated as zero. LTB drops the policy.
func registerTheMasterOfKeys(r *Registry) {
	r.OnETB("The Master of Keys", theMasterOfKeysETB)
	r.OnTrigger("The Master of Keys", "permanent_ltb", theMasterOfKeysLTB)
}

func theMasterOfKeysLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
}

// theMasterOfKeysEnchantmentPredicate filters cards eligible for the
// graveyard-escape grant. Defined at file scope so every Master of Keys
// ETB shares the same predicate pointer.
func theMasterOfKeysEnchantmentPredicate(c *gameengine.Card) bool {
	if c == nil {
		return false
	}
	return cardHasType(c, "enchantment")
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
	// R58: register the enchantment-escape grant on the controller's
	// graveyard. ManaCost=-1 means "use the card's printed mana cost";
	// ExileOnResolve=true routes the resolved spell to exile per
	// escape's printed rider.
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "the_master_of_keys_enchantment_escape",
		Zone:            gameengine.ZoneGraveyard,
		OwnerScope:      "self",
		CasterScope:     "controller",
		ControllerSeat:  perm.Controller,
		Predicate:       theMasterOfKeysEnchantmentPredicate,
		ManaCost:        -1,
		ExileOnResolve:  true,
		Duration:        "while_source_on_bf",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     seatIdx,
		"x":        x,
		"counters": x,
		"milled":   milled,
		"policy":   "the_master_of_keys_enchantment_escape",
	})
}
