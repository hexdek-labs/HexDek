package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerQuilledGreatwurm wires Quilled Greatwurm (Muninn parser-gap rank
// ~142, combat-damage +1/+1 anthem + graveyard alt-cost).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{4}{G}{G}
//	Creature — Wurm
//	Trample
//	Whenever a creature you control deals combat damage during your turn,
//	put that many +1/+1 counters on it. (It must survive to get the
//	counters.)
//	You may cast this card from your graveyard by removing six counters
//	from among creatures you control in addition to paying its other
//	costs.
//
// Implementation:
//   - Trample via AST keyword pipeline.
//   - OnTrigger("combat_damage_dealt"): gated on damager_seat == controller
//     AND active_seat == controller ("during your turn"). Apply N +1/+1
//     counters where N = damage amount, but only if the damager is still
//     alive (engine fires combat_damage_dealt after lethal-damage marking
//     but before SBAs sweep — a dying damager still has the perm pointer,
//     so we check Toughness() > DamageMarked before counter-application
//     to mirror "must survive to get the counters"). Includes self-damage
//     from Quilled Greatwurm itself.
//   - Graveyard alt-cost is a static cost replacement (remove counters
//     instead of paying part of {4}{G}{G}). The engine has no per-card
//     hook into spell-casting cost calculation for graveyard-cast paths
//     yet — emitPartial.
func registerQuilledGreatwurm(r *Registry) {
	r.OnTrigger("Quilled Greatwurm", "combat_damage_dealt", quilledGreatwurmCombatDamage)
	r.OnETB("Quilled Greatwurm", quilledGreatwurmETB)
}

func quilledGreatwurmETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "quilled_greatwurm_graveyard_cost_partial"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"graveyard_alt_cost_remove_six_counters_unwired_pending_graveyard_cast_cost_hook")
}

func quilledGreatwurmCombatDamage(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "quilled_greatwurm_counter_grow"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires combat_damage_player with source_seat + source_card +
	// defender_seat + amount. Legacy ctx["damager_seat"]/["damager_perm"]
	// are read as fallbacks.
	dmgSeat, ok := ctx["source_seat"].(int)
	if !ok {
		dmgSeat, _ = ctx["damager_seat"].(int)
	}
	if dmgSeat != perm.Controller {
		return
	}
	// "During your turn" — gate on the active player being the controller.
	if gs.Active != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	damager, _ := ctx["damager_perm"].(*gameengine.Permanent)
	// Engine doesn't thread damager_perm; look up the damaging creature on
	// the source seat's battlefield by source_card name when missing.
	if damager == nil {
		srcName, _ := ctx["source_card"].(string)
		if srcName == "" {
			return
		}
		seat := gs.Seats[dmgSeat]
		if seat == nil {
			return
		}
		for _, p := range seat.Battlefield {
			if p != nil && p.Card != nil && p.Card.DisplayName() == srcName {
				damager = p
				break
			}
		}
	}
	if damager == nil || damager.Card == nil || !damager.IsCreature() {
		return
	}
	if damager.Controller != perm.Controller {
		return
	}
	// "Must survive to get the counters" — skip if marked lethal.
	if damager.MarkedDamage >= damager.Toughness() {
		return
	}
	damager.AddCounter("+1/+1", amount)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"damager":  damager.Card.DisplayName(),
		"counters": amount,
	})
}
