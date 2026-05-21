package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTovolarDireOverlord wires Tovolar, Dire Overlord (DFC front).
//
// Oracle text (front: Tovolar, Dire Overlord):
//
//	{1}{R}{G}
//	Legendary Creature — Human Werewolf
//	Whenever a Wolf or Werewolf you control deals combat damage to a
//	  player, draw a card.
//	At the beginning of your upkeep, if you control three or more Wolves
//	  and/or Werewolves, it becomes night. Then transform any number of
//	  Human Werewolves you control.
//	Daybound
//
// Back (Tovolar, the Midnight Scourge):
//
//	Whenever a Wolf or Werewolf you control deals combat damage to a
//	  player, draw a card.
//	{X}{R}{G}: Target Wolf or Werewolf you control gets +X/+0 and gains
//	  trample until end of turn.
//	Nightbound
//
// Implementation:
//   - "creature_combat_damage_to_player" trigger gated to attacker
//     being controlled by Tovolar's controller and a Wolf/Werewolf:
//     draw a card.
//   - Upkeep trigger gated to active_seat == controller AND day/night
//     state: count Wolves/Werewolves and flip game-state to night via
//     gs.Flags["is_night"] = 1. Transformation of Human Werewolves is
//     emitPartial.
//   - Daybound/Nightbound transformation is engine-side state.
func registerTovolarDireOverlord(r *Registry) {
	r.OnTrigger("Tovolar, Dire Overlord", "creature_combat_damage_to_player", tovolarOnCombatDamage)
	r.OnTrigger("Tovolar, Dire Overlord // Tovolar, the Midnight Scourge", "creature_combat_damage_to_player", tovolarOnCombatDamage)
	r.OnTrigger("Tovolar, Dire Overlord", "upkeep_controller", tovolarUpkeepNight)
	r.OnTrigger("Tovolar, Dire Overlord // Tovolar, the Midnight Scourge", "upkeep_controller", tovolarUpkeepNight)
}

func tovolarOnCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tovolar_combat_damage_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil {
		return
	}
	if atk.Controller != perm.Controller {
		return
	}
	if atk.Card == nil {
		return
	}
	if !cardHasTypeAny(atk.Card, "wolf", "werewolf") {
		return
	}
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"attacker": atk.Card.DisplayName(),
	})
}

func tovolarUpkeepNight(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "tovolar_upkeep_night"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasTypeAny(p.Card, "wolf", "werewolf") {
			count++
		}
	}
	if count < 3 {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["is_night"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"count": count,
	})
	// Human-Werewolf transformation rider is wired in
	// custom_tovolar_dire_overlord.go (R50 batchH) — runs after we
	// flip is_night and calls TransformPermanent on every own DFC
	// Human Werewolf front face.
}
