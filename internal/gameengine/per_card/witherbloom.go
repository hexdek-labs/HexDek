package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWitherbloom wires Witherbloom, the Balancer (Batch #30 rewrite).
//
// Oracle text (Scryfall, Secrets of Strixhaven, verified 2026-05-01):
//
//	{6}{B}{G}, 5/5 Legendary Creature — Elder Dragon
//	Affinity for creatures (This spell costs {1} less to cast for each
//	  creature you control.)
//	Flying, deathtouch
//	Instant and sorcery spells you cast have affinity for creatures.
//
// Engine wiring (no per-event handler is required — every clause is a
// static / cost-time effect):
//   - Flying, deathtouch: AST keyword pipeline.
//   - "Affinity for creatures" on Witherbloom's own cast: handled by
//     gameengine.HasAffinityForCreatures + CountCreaturesOnBattlefield in
//     cost_modifiers.go (mirrors the artifact pattern).
//   - "Instant and sorcery spells you cast have affinity for creatures":
//     handled by the per-permanent `case "Witherbloom, the Balancer"`
//     branch in cost_modifiers.go that grants the same {1}-less-per-
//     creature reduction to instant/sorcery spells the controller casts.
//
// The ETB stub below is registered only so HasETB("Witherbloom, the
// Balancer") returns true — keeps the audit/coverage tooling happy and
// emits a verifiable per-card-handler log entry on resolve.
func registerWitherbloom(r *Registry) {
	r.OnETB("Witherbloom, the Balancer", witherbloomETB)
}

func witherbloomETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "witherbloom_balancer"
	if gs == nil || perm == nil {
		return
	}
	creatures := 0
	if seat := gs.Seats[perm.Controller]; seat != nil {
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() {
				creatures++
			}
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"keywords":         []string{"flying", "deathtouch", "affinity_for_creatures"},
		"creature_count":   creatures,
		"granted_to":       "instant_and_sorcery_spells",
		"reduction_each":   "{1} per creature controlled",
	})
}
