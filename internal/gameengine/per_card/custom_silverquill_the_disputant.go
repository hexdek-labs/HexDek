package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSilverquillTheDisputantCustom wires the ETB drain. The
// auto-generated gen_silverquill_the_disputant.go was a partial-emit
// stub; both the file and its batch_generated.go register call were
// deleted in Versailles Phase 2I — this is now the sole registration
// site, called from registry.go::registerDefaults.
//
// Oracle text (Strixhaven / Commander 2021, {3}{W}{B}):
//
//	Flying, lifelink
//	When Silverquill, the Disputant enters the battlefield, target
//	opponent loses 3 life and you gain 3 life. Then if Silverquill
//	entered the battlefield from the command zone, you may attach
//	target Aura or Equipment you control to it.
//
// Implementation:
//   - OnETB: pick the highest-life living opponent; drain 3, gain 3.
//   - The "if from command zone" attach clause is modeled as
//     emitPartial — Aura/Equipment attachment isn't on the per-card hook
//     path. Tracking from-command-zone entry is a Permanent.Flags
//     concern owned by the cast pipeline.
//   - Flying, lifelink — AST keyword pipeline.
func registerSilverquillTheDisputantCustom(r *Registry) {
	r.OnETB("Silverquill, the Disputant", silverquillETBDrain)
}

func silverquillETBDrain(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "silverquill_the_disputant_etb_drain"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}

	// Pick highest-life living opponent (most mana-efficient drain target).
	target := -1
	bestLife := -1 << 30
	for _, opp := range gs.Opponents(seatIdx) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target >= 0 {
		gameengine.LoseLife(gs, target, 3, perm.Card.DisplayName())
	}
	gameengine.GainLife(gs, seatIdx, 3, perm.Card.DisplayName())

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seatIdx,
		"target_seat":   target,
		"opp_lost_life": 3,
		"you_gained":    3,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"command_zone_etb_attach_aura_or_equipment_clause_not_modeled")
	_ = gs.CheckEnd()
}
