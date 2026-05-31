package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGlissaSunslayer wires Glissa Sunslayer.
//
// Oracle text (Scryfall, verified 2026-05-30, {1}{B}{G}, 3/3
// Legendary Creature — Phyrexian Zombie Elf):
//
//	First strike, deathtouch
//	Whenever Glissa Sunslayer deals combat damage to a player, choose
//	one —
//	  • You draw a card and lose 1 life.
//	  • Destroy target enchantment.
//	  • Remove up to three counters from target permanent.
//
// Implementation (R60 stub sweep):
//   - AI mode selection: prefer draw-and-lose-1 when library has cards
//     (cheap card advantage). Fall through to destroy-target-enchantment
//     if library is empty and any opponent controls an enchantment
//     (avoids decking ourselves on the cantrip). Final fallback to the
//     remove-counters mode is still partial — counter removal needs
//     target selection across all counter types.
//   - Library-empty fallback prevents the "draw from empty deck → take
//     a §704.5b loss" interaction on Glissa-deck mirrors.
func registerGlissaSunslayer(r *Registry) {
	r.OnTrigger("Glissa Sunslayer", "combat_damage_player", glissaSunslayerCombatDamage)
}

func glissaSunslayerCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "glissa_sunslayer_modal"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	sourceName, _ := ctx["source_card"].(string)
	if sourceName != "" && sourceName != perm.Card.DisplayName() {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Mode 1: draw a card and lose 1 life — preferred when library has cards.
	if len(seat.Library) > 0 {
		card := seat.Library[0]
		seat.Library = seat.Library[1:]
		seat.Hand = append(seat.Hand, card)
		gameengine.LoseLife(gs, perm.Controller, 1, perm.Card.DisplayName())
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat": perm.Controller,
			"mode": "draw_lose1",
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"remove_counters_mode_unimplemented")
		return
	}

	// Mode 2: destroy target enchantment — empty-library fallback. Pick
	// the first opponent's enchantment we find.
	for _, opp := range gs.Opponents(perm.Controller) {
		oppSeat := gs.Seats[opp]
		if oppSeat == nil || oppSeat.Lost {
			continue
		}
		for _, p := range oppSeat.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if !cardHasType(p.Card, "enchantment") {
				continue
			}
			targetName := p.Card.DisplayName()
			if gameengine.DestroyPermanent(gs, p, perm) {
				emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
					"seat":   perm.Controller,
					"mode":   "destroy_enchantment",
					"target": targetName,
				})
				return
			}
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"mode": "no_legal_mode",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"remove_counters_mode_unimplemented")
}
