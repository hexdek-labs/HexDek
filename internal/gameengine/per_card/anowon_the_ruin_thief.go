package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAnowonTheRuinThief wires Anowon, the Ruin Thief.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Other Rogues you control get +1/+1.
//	Whenever one or more Rogues you control deal combat damage to a player,
//	that player mills cards equal to the damage dealt. If a creature card
//	is milled this way, you draw a card.
//
// Implementation:
//   - OnETB: emitPartial for the lord effect (+1/+1 to other Rogues).
//     Static P/T buff applied via applyTribalBuff for the existing Rogues
//     on the battlefield at ETB time. Rogues entering after ETB are not
//     retroactively buffed (static continuous layer not fully modeled).
//   - OnTrigger "combat_damage_player": fires whenever any creature
//     controlled by Anowon's controller deals combat damage to a player.
//     If the source creature is a Rogue, the damaged player mills X cards
//     (where X = damage amount). If any milled card has type "creature",
//     Anowon's controller draws a card.
func registerAnowonTheRuinThief(r *Registry) {
	r.OnETB("Anowon, the Ruin Thief", anowonTheRuinThiefETB)
	r.OnTrigger("Anowon, the Ruin Thief", "combat_damage_player", anowonTheRuinThiefCombatDamage)
}

func anowonTheRuinThiefETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "anowon_ruin_thief_lord"
	if gs == nil || perm == nil {
		return
	}
	// Apply +1/+1 to other Rogues currently on the battlefield.
	applyTribalBuff(gs, perm, "rogue", 1, 1, "Anowon, the Ruin Thief")
	// Flag that post-ETB Rogue entries are not continuously buffed.
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"lord_buff_applies_only_at_etb_not_continuously_to_later_rogues")
}

func anowonTheRuinThiefCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "anowon_ruin_thief_mill"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	sourceSeat, _ := ctx["source_seat"].(int)
	sourceName, _ := ctx["source_card"].(string)
	defenderSeat, _ := ctx["defender_seat"].(int)
	amount, _ := ctx["amount"].(int)

	// Only fires when the source creature is controlled by Anowon's controller.
	if sourceSeat != perm.Controller {
		return
	}
	if amount <= 0 {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	// Check that the source creature is a Rogue.
	isRogue := false
	for _, p := range gs.Seats[sourceSeat].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Card.DisplayName() != sourceName {
			continue
		}
		// Check Card.Types first; fall back to TypeLine.
		for _, t := range p.Card.Types {
			if strings.EqualFold(t, "rogue") {
				isRogue = true
				break
			}
		}
		if !isRogue && strings.Contains(strings.ToLower(p.Card.TypeLine), "rogue") {
			isRogue = true
		}
		break
	}
	if !isRogue {
		return
	}

	defender := gs.Seats[defenderSeat]
	if defender == nil {
		return
	}

	// Mill X cards from the defender's library (X = damage amount).
	milled := 0
	creatureMilled := false
	for i := 0; i < amount; i++ {
		if len(defender.Library) == 0 {
			break
		}
		top := defender.Library[0]
		gameengine.MoveCard(gs, top, defenderSeat, "library", "graveyard", "anowon_mill")
		milled++
		gs.LogEvent(gameengine.Event{
			Kind:   "mill",
			Seat:   perm.Controller,
			Target: defenderSeat,
			Source: perm.Card.DisplayName(),
			Amount: 1,
			Details: map[string]interface{}{
				"card":   top.DisplayName(),
				"reason": slug,
			},
		})
		if cardHasType(top, "creature") {
			creatureMilled = true
		}
	}

	// Draw a card if any milled card was a creature.
	drew := false
	if creatureMilled && milled > 0 {
		drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())
		drew = drawn != nil
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"source_card":     sourceName,
		"defender_seat":   defenderSeat,
		"damage":          amount,
		"milled":          milled,
		"creature_milled": creatureMilled,
		"drew":            drew,
	})
}
