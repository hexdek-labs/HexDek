package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCarmenCruelSkymarcher wires Carmen, Cruel Skymarcher.
//
// Oracle text:
//
//	Flying
//	Whenever a player sacrifices a permanent, put a +1/+1 counter on
//	Carmen and you gain 1 life.
//	Whenever Carmen attacks, return up to one target permanent card
//	with mana value less than or equal to Carmen's power from your
//	graveyard to the battlefield.
func registerCarmenCruelSkymarcher(r *Registry) {
	r.OnTrigger("Carmen, Cruel Skymarcher", "permanent_sacrificed", carmenSacTrigger)
	r.OnTrigger("Carmen, Cruel Skymarcher", "attacks", carmenAttacks)
}

func carmenSacTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "carmen_sacrifice_growth"
	if gs == nil || perm == nil {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func carmenAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "carmen_attack_reanimate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires creature_attacks with attacker_perm + attacker_seat +
	// attacker_card. Legacy "seat" key kept as a fallback. Carmen's printed
	// trigger is "Whenever Carmen attacks" so gate on attacker_perm == perm
	// when available; fall back to the seat-only filter otherwise.
	if atk, ok := ctx["attacker_perm"].(*gameengine.Permanent); ok && atk != nil {
		if atk != perm {
			return
		}
	} else {
		attackerSeat, ok := ctx["attacker_seat"].(int)
		if !ok {
			attackerSeat, _ = ctx["seat"].(int)
		}
		if attackerSeat != perm.Controller {
			return
		}
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	pow := perm.Power()
	bestIdx := -1
	bestCMC := -1
	for i, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") && !cardHasType(c, "artifact") &&
			!cardHasType(c, "enchantment") && !cardHasType(c, "planeswalker") &&
			!cardHasType(c, "land") && !cardHasType(c, "battle") {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc <= pow && cmc > bestCMC {
			bestCMC = cmc
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_target", map[string]interface{}{
			"seat":   perm.Controller,
			"max_cmc": pow,
		})
		return
	}
	card := seat.Graveyard[bestIdx]
	gameengine.MoveCard(gs, card, perm.Controller, "graveyard", "battlefield", "carmen_attack")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"reanimated": card.DisplayName(),
		"cmc":       bestCMC,
	})
}
