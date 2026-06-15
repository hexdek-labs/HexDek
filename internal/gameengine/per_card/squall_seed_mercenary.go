package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSquallSeedMercenary wires Squall, SeeD Mercenary.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	Rough Divide — Whenever a creature you control attacks alone, it
//	gains double strike until end of turn.
//	Whenever Squall deals combat damage to a player, return target
//	permanent card with mana value 3 or less from your graveyard to
//	the battlefield.
//
// Implementation:
//   - "declare_attackers" trigger: if exactly one creature is attacking
//     for Squall's controller and that creature is one we control,
//     grant it double strike (kw flag + eot flag) for the turn.
//   - "combat_damage_player" trigger: when Squall deals combat damage
//     to a player, reanimate the highest-CMC permanent (CMC <= 3) from
//     graveyard. Picks the most-impactful eligible card.
func registerSquallSeedMercenary(r *Registry) {
	r.OnTrigger("Squall, SeeD Mercenary", "declare_attackers", squallSeedAttackAlone)
	r.OnTrigger("Squall, SeeD Mercenary", "combat_damage_player", squallSeedCombatDamage)
}

func squallSeedAttackAlone(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "squall_seed_attack_alone_double_strike"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// CR §508.4a — "attacks alone" counts only DECLARED attackers; §506.3
	// entered-attacking creatures (tokens, ninjutsu) don't count. Use
	// WasDeclaredAttacker() so this agrees with exalted (combat.go
	// `len(declared)==1`) rather than diverging via IsAttacking().
	var lone *gameengine.Permanent
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.WasDeclaredAttacker() {
			count++
			lone = p
		}
	}
	if count != 1 || lone == nil {
		return
	}
	if lone.Flags == nil {
		lone.Flags = map[string]int{}
	}
	lone.Flags["kw:double_strike"] = 1
	lone.Flags["double_strike_eot"] = 1
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": lone.Card.DisplayName(),
	})
}

func squallSeedCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "squall_seed_reanimate_three_or_less"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcName, _ := ctx["source_card"].(string)
	if !strings.EqualFold(srcName, perm.Card.DisplayName()) {
		return
	}
	srcSeat, _ := ctx["source_seat"].(int)
	if srcSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Card
	bestCMC := -1
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if !cardHasType(c, "creature") && !cardHasType(c, "artifact") &&
			!cardHasType(c, "enchantment") && !cardHasType(c, "planeswalker") &&
			!cardHasType(c, "land") {
			continue
		}
		cm := cardCMC(c)
		if cm > 3 {
			continue
		}
		if cm > bestCMC {
			bestCMC = cm
			best = c
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_eligible_target", nil)
		return
	}
	gameengine.MoveCard(gs, best, perm.Controller, "graveyard", "battlefield", "squall_seed_reanimate")
	createPermanent(gs, perm.Controller, best, false)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"reanimated": best.DisplayName(),
	})
}
