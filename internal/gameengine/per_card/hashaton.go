package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHashaton wires Hashaton, Scarab's Fist.
//
// Oracle text:
//
//	Whenever you discard a creature card, you may pay {2}{U}. If you
//	do, create a tapped token that's a copy of that card, except it's
//	a 4/4 black Zombie.
//
// Implementation:
//   - Listens on "card_discarded"; gates on discarder_seat == controller
//     and discarded card type == creature.
//   - Cost check: requires the controller to have at least 3 mana
//     available in their pool ({2}{U} ≈ 3 generic mana for the AI's
//     simplified mana model). If insufficient, emit a fail event and
//     bail.
//   - Token creation: deep-copy the discarded card, override Types so the
//     token is a {creature, token, zombie} with black color pip, and
//     stamp BasePower/BaseToughness to 4/4. Token enters tapped.
func registerHashaton(r *Registry) {
	r.OnTrigger("Hashaton, Scarab's Fist", "card_discarded", hashatonDiscardTrigger)
}

func hashatonDiscardTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hashaton_discard_zombie_copy"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat != perm.Controller {
		return
	}
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil {
		return
	}
	if !cardHasType(card, "creature") {
		return
	}

	// Optional cost {2}{U}. Use the seat's mana pool as a coarse gate so
	// we don't auto-fire on every discard — the AI will hold mana for
	// Hashaton when it has the option.
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	const cost = 3
	if seat.ManaPool < cost {
		emitFail(gs, slug, perm.Card.DisplayName(), "insufficient_mana", map[string]interface{}{
			"seat":      perm.Controller,
			"discarded": card.DisplayName(),
			"required":  cost,
			"available": seat.ManaPool,
		})
		return
	}
	seat.ManaPool -= cost

	// Build the token: copy of that card, except 4/4 black Zombie.
	token := card.DeepCopy()
	token.Owner = perm.Controller
	token.IsCopy = true
	// Force creature/token/zombie typing and a black color pip while
	// keeping any non-conflicting subtypes (the original creature types
	// stay so combat and tribal interactions still work).
	hasToken := false
	hasZombie := false
	hasCreature := false
	hasBlackPip := false
	filtered := token.Types[:0]
	for _, t := range token.Types {
		switch t {
		case "token":
			hasToken = true
		case "zombie":
			hasZombie = true
		case "creature":
			hasCreature = true
		case "pip:B":
			hasBlackPip = true
		case "pip:W", "pip:U", "pip:R", "pip:G", "pip:C":
			// Drop original color pips — token is black.
			continue
		}
		filtered = append(filtered, t)
	}
	token.Types = filtered
	if !hasCreature {
		token.Types = append(token.Types, "creature")
	}
	if !hasToken {
		token.Types = append(token.Types, "token")
	}
	if !hasZombie {
		token.Types = append(token.Types, "zombie")
	}
	if !hasBlackPip {
		token.Types = append(token.Types, "pip:B")
	}
	token.Colors = []string{"B"}
	token.BasePower = 4
	token.BaseToughness = 4

	enterBattlefieldWithETB(gs, perm.Controller, token, true /*tapped*/)

	gs.LogEvent(gameengine.Event{
		Kind:   "create_token",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"slug":      slug,
			"token":     token.DisplayName(),
			"copy_of":   card.DisplayName(),
			"power":     4,
			"tough":     4,
			"tapped":    true,
			"reason":    "hashaton_discard_copy",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"discarded": card.DisplayName(),
		"copy":      token.DisplayName(),
		"cost_paid": cost,
	})
}
