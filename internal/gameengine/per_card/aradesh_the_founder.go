package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAradeshTheFounder wires Aradesh, the Founder
// (Muninn parser-gap #100, ~3.2K hits).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Aradesh, the Founder — {2}{W} Legendary Creature — Human Soldier 3/3
//	Enlist (As this creature attacks, you may tap a nonattacking
//	creature you control without summoning sickness. When you do, add
//	its power to this creature's until end of turn.)
//	Whenever a creature you control attacks, if it enlisted a creature
//	this combat, the creature that attacked gains double strike until
//	end of turn. If that creature's power is 4 or greater, draw a card.
//
// Implementation:
//   - creature_attacks: if attacker is ours AND attacker.Flags["enlisted_this_combat"]
//     is set, stamp double_strike for the turn. If attacker.Card.BasePower
//     (+ temp_power + counters) >= 4, draw a card.
//   - The enlist mechanic itself (cost replacement to tap a creature,
//     +X/+0 stamp) is an attack-declaration concern — emitPartial.
func registerAradeshTheFounder(r *Registry) {
	r.OnTrigger("Aradesh, the Founder", "creature_attacks", aradeshOnCreatureAttacks)
	r.OnETB("Aradesh, the Founder", aradeshETB)
}

func aradeshETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	emit(gs, "aradesh_etb", "Aradesh, the Founder", map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "aradesh", "Aradesh, the Founder",
		"enlist_keyword_attack_declaration_hook_unimplemented")
}

func aradeshOnCreatureAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aradesh_creature_attacks"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attacker, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if attacker == nil || attacker.Controller != perm.Controller {
		return
	}
	if attacker.Flags == nil || attacker.Flags["enlisted_this_combat"] == 0 {
		emit(gs, slug, "Aradesh, the Founder", map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"reason":    "no_enlist_this_combat",
		})
		return
	}
	if attacker.Flags == nil {
		attacker.Flags = map[string]int{}
	}
	attacker.Flags["kw:double_strike"] = 1
	// Use the canonical current power (counters + until-EOT modifications +
	// layer effects), not a hand-rolled BasePower+temp_power+counter sum that
	// missed the enlist +power/+0 modification (and any other power source).
	power := attacker.Power()
	drew := false
	if power >= 4 {
		drew = drawOne(gs, perm.Controller, "Aradesh, the Founder") != nil
	}
	emit(gs, slug, "Aradesh, the Founder", map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"attacker":  attacker.Card.DisplayName(),
		"power":     power,
		"drew":      drew,
	})
}
