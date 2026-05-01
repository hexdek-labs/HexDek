package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHinataDawnCrowned wires Hinata, Dawn-Crowned. Batch #33.
//
// Oracle text (Kamigawa: Neon Dynasty, {1}{U}{R}{W}, Legendary
// Creature — Kirin Spirit, 4/4):
//
//	Flying, trample
//	Spells you cast cost {1} less to cast for each target.
//	Spells your opponents cast cost {1} more to cast for each target.
//
// Implementation:
//   - Flying, trample: AST keywords.
//   - The two cost-modification clauses are wired in
//     gameengine/cost_modifiers.go's ScanCostModifiers switch under the
//     "Hinata, Dawn-Crowned" case. The amount is the number of target
//     clauses in the spell's oracle text (CountTargetClauses), applied
//     as a CostModReduction when the spell is cast by Hinata's
//     controller and as a CostModIncrease when cast by an opponent.
//   - This file registers an ETB hook to log the wiring confirmation
//     (and emit a partial-effect note) so spectators see Hinata is
//     active, mirroring how Witherbloom records its granted-affinity
//     coverage at ETB time.
func registerHinataDawnCrowned(r *Registry) {
	r.OnETB("Hinata, Dawn-Crowned", hinataETB)
}

func hinataETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "hinata_dawn_crowned_target_cost_mod_active"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"effect": "self_spells_cost_1_less_per_target_opp_spells_cost_1_more_per_target",
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"target_count_estimated_from_oracle_text_undercounts_multi_target_spells")
}
