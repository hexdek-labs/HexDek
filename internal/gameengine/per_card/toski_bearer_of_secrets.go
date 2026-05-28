package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerToski wires Toski, Bearer of Secrets.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Toski%2C%20Bearer%20of%20Secrets):
//
//	Toski, Bearer of Secrets can't be the target of spells or abilities
//	your opponents control.
//	Whenever a creature you control deals combat damage to a player,
//	draw a card.
//
// {2}{G} 1/1 Legendary Creature — Squirrel. Indestructible. Mono-green
// card-advantage commander. Every connected hit (token swarm, single
// evasive creature, anything that gets through) cantrips. Pairs with
// any "go-wide" or unblockable shell.
//
// The targeting protection is a static effect handled at target-selection
// layer; not part of this trigger handler (the engine's PickTarget pass
// reads the `cant_be_targeted_by_opponents` flag elsewhere). This file
// owns only the combat-damage cantrip.
//
// Implementation:
//   - OnTrigger("combat_damage_player"). Filter: source_seat ==
//     perm.Controller AND defender_seat != perm.Controller AND the
//     dealing creature is on perm.Controller's battlefield (same
//     pattern as Tymna's per-seat hit observer).
//   - Action: draw 1 per qualifying damage event.
func registerToski(r *Registry) {
	r.OnTrigger("Toski, Bearer of Secrets", "combat_damage_player", toskiCombatDamage)
}

func toskiCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toski_bearer_of_secrets"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	sourceSeat, _ := ctx["source_seat"].(int)
	defenderSeat, _ := ctx["defender_seat"].(int)
	if sourceSeat != perm.Controller {
		return
	}
	if defenderSeat == perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	drawOne(gs, perm.Controller, "Toski, Bearer of Secrets")

	emit(gs, slug, "Toski, Bearer of Secrets", map[string]interface{}{
		"seat":          perm.Controller,
		"defender_seat": defenderSeat,
	})
}
