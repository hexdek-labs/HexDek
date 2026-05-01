package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDoranBesiegedByTime wires Doran, Besieged by Time.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Each creature spell you cast with toughness greater than its power
//	costs {1} less to cast.
//	Whenever a creature you control attacks or blocks, it gets +X/+X
//	until end of turn, where X is the difference between its power and
//	its toughness.
//
// NOT to be confused with "Doran, the Siege Tower" — different commander.
//
// Implementation:
//   - The {1}-less cost reduction lives in cost_modifiers.go's
//     ScanCostModifiers switch (case "Doran, Besieged by Time"), gated
//     on isSelf + isCreature + card.BaseToughness > card.BasePower.
//   - OnTrigger("creature_attacks"): when a creature controlled by Doran's
//     controller attacks, compute X = toughness - power; if X > 0 grant
//     +X/+X EOT via a Modification entry (CR §613 layer 7c approximation).
//   - The "or blocks" half is not yet wired through the per_card framework
//     — there is no engine-level "creature_blocks" card-trigger event
//     (block triggers go through observer_triggers.fireBlockTriggers,
//     which only fires AST-driven self-triggers). Logged via emitPartial
//     so audits flag this gap.
func registerDoranBesiegedByTime(r *Registry) {
	r.OnETB("Doran, Besieged by Time", doranBesiegedByTimeETB)
	r.OnTrigger("Doran, Besieged by Time", "creature_attacks", doranBesiegedByTimeAttackBuff)
}

func doranBesiegedByTimeETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "doran_besieged_by_time_static"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":               perm.Controller,
		"cost_reduction":     1,
		"reduction_filter":   "creature_spells_with_toughness_gt_power",
		"cost_wired_in":      "cost_modifiers.go",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(), "blocks_half_no_block_trigger_event")
}

func doranBesiegedByTimeAttackBuff(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "doran_besieged_by_time_attack_buff"
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
	x := atk.Toughness() - atk.Power()
	if x <= 0 {
		return
	}
	atk.Modifications = append(atk.Modifications, gameengine.Modification{
		Power:     x,
		Toughness: x,
		Duration:  "until_end_of_turn",
		Timestamp: gs.NextTimestamp(),
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"attacker": atk.Card.DisplayName(),
		"x":        x,
		"buff":     "+X/+X_eot",
	})
}
