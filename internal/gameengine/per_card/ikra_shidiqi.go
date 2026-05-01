package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerIkraShidiqi wires Ikra Shidiqi, the Usurper.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Menace
//	Whenever a creature you control deals combat damage to a player,
//	you gain life equal to that creature's toughness.
//	Partner (You can have two commanders if both have partner.)
//
// Menace and Partner are intrinsic keywords parsed by the AST; this
// handler only implements the lifegain trigger.
//
// Implementation:
//   - Listens on combat_damage_player. The source must be controlled by
//     Ikra's controller. We resolve the source's current toughness from
//     the live battlefield permanent (not the printed value) so +1/+1
//     counters and other static modifiers are reflected. CR §120.3:
//     "amount of life gained ... is determined as the ability resolves."
//   - Falls back to BaseToughness on the card if the source has already
//     left the battlefield by the time the trigger resolves.
func registerIkraShidiqi(r *Registry) {
	r.OnTrigger("Ikra Shidiqi, the Usurper", "combat_damage_player", ikraShidiqiTrigger)
}

func ikraShidiqiTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ikra_shidiqi_combat_damage_lifegain"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	if sourceSeat != perm.Controller {
		return
	}
	if sourceName == "" {
		return
	}

	toughness := 0
	found := false
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !strings.EqualFold(p.Card.DisplayName(), sourceName) {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		toughness = p.Toughness()
		found = true
		break
	}
	if !found {
		// Source no longer on battlefield (e.g. simul-died); use
		// last-known characteristics from the dying card if the engine
		// preserved it in ctx.
		if card, ok := ctx["source_card_obj"].(*gameengine.Card); ok && card != nil {
			toughness = card.BaseToughness
		}
	}
	if toughness <= 0 {
		return
	}

	gameengine.GainLife(gs, perm.Controller, toughness, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"source_card": sourceName,
		"life_gained": toughness,
	})
}
