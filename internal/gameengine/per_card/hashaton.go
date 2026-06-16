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
	gameengine.SyncManaAfterSpend(seat)
	// CR §702 optional trigger cost — credit it to any in-flight legality
	// window for this seat (extort/ward/Esper-Sentinel aux-spend class,
	// legality.go NoteManaSpend). Hashaton's card_discarded trigger resolves
	// INSIDE the discard window of whatever caused the discard (e.g. Tireless
	// Tribe's "Discard a card" activation cost); without this credit the
	// {2}{U} drain reads as that action over-paying its §601.2f announced
	// total — the live grinder "activate Tireless Tribe#0 over-paid" firespot.
	gs.Legality.NoteManaSpend(perm.Controller, cost)

	// Build the token: copy of that card, except 4/4 black Zombie.
	// Phase 5 chokepoint: route through MintTokenAsCopyOf so the token's
	// InstanceID is freshly minted (TK provenance) instead of inheriting
	// the discarded card's OG ID. Template overrides (force creature +
	// zombie subtype, mono-black, 4/4) are applied AFTER the mint.
	token := gameengine.MintTokenAsCopyOf(gs, card, perm.Controller, gameengine.CurrentMintEnablerID(gs))
	if token == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "mint_token_returned_nil", nil)
		return
	}
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
			// Drop original color pips — Hashaton's token is mono-black.
			// `pip:C` (colorless) is included for Eldrazi reanimations
			// (Emrakul, Ulamog, Kozilek): their Card.Types arrays
			// carry the C pip and we still need to strip it. Phase-1D
			// audit flagged pip:C as unreachable because pip markers
			// come from Card.Types JSON data, not Go source — the
			// static scanner can't see emitter coverage there.
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
