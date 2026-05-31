package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Saskia the Unyielding — {1}{R}{G}{W}{B} 3/4 Legendary Creature — Human
// Soldier.
//
//   Vigilance, haste
//   As Saskia enters, choose a player.
//   Whenever a creature you control deals combat damage to a player, it
//   deals that much damage to the chosen player.
//
// Implementation:
//   - OnETB: pick a target player (highest-life non-Lost opponent — the
//     long-game threat). Store as perm.Flags["saskia_target_seat"] with
//     the +1 offset convention (0 means unset; 1+ = seat-1).
//   - OnTrigger combat_damage_player: filter ctx["source_seat"] ==
//     Saskia.Controller; if ctx["defender_seat"] != Saskia's chosen
//     target, DealDamage(chosenSeat, amount). The redirect is ADDITIONAL
//     ("it deals that much damage to the chosen player"), not a replace.

func init() {
	registerSaskiaTheUnyielding(Global())
	AddResetHook(registerSaskiaTheUnyielding)
}

func registerSaskiaTheUnyielding(r *Registry) {
	r.OnETB("Saskia the Unyielding", saskiaETB)
	r.OnTrigger("Saskia the Unyielding", "combat_damage_player", saskiaCombatDamage)
}

func saskiaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "saskia_choose_player"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	pickSeat := -1
	bestLife := -9999
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == perm.Controller {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			pickSeat = i
		}
	}
	if pickSeat < 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_opponent_to_choose",
		})
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["saskia_target_seat"] = pickSeat + 1 // +1 offset; 0 means unset
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"chosen_player": pickSeat,
	})
}

func saskiaCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "saskia_redirect_damage"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	defenderSeat, _ := ctx["defender_seat"].(int)
	stored := perm.Flags["saskia_target_seat"]
	if stored == 0 {
		return // no player chosen
	}
	chosenSeat := stored - 1
	if chosenSeat == defenderSeat {
		return // already taking damage as the defender — no double-up
	}
	if chosenSeat < 0 || chosenSeat >= len(gs.Seats) {
		return
	}
	if gs.Seats[chosenSeat] == nil || gs.Seats[chosenSeat].Lost {
		return
	}
	gameengine.DealDamage(gs, chosenSeat, amount, perm.Card.DisplayName()+" (Saskia redirect)")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"chosen_player": chosenSeat,
		"amount":        amount,
		"original_def":  defenderSeat,
	})
}
