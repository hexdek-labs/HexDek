package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhyrexianDreadnought wires Phyrexian Dreadnought (Muninn
// snowflake first seen 2026-05-15, +1/turn).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Phyrexian Dreadnought — {1} Artifact Creature — Phyrexian Dreadnought 12/12
//	Trample
//	When this creature enters, sacrifice it unless you sacrifice any
//	number of creatures with total power 12 or greater.
//
// Implementation:
//   - OnETB: scan controller's other creatures' printed (+ pumped)
//     power. If the AI/player flag "dreadnought_pay_etb" is set, assume
//     the cost was paid (Stiflexible decks pay 0 with Stifle/Trickbind
//     anyway). Otherwise, if no creature you control has any power, or
//     total available power < 12, sacrifice Dreadnought.
//   - We do NOT pick which creatures to sacrifice here — that decision
//     belongs to the AI's payment-decision hook. emitPartial covers the
//     "sacrifice any number of creatures with total power 12 or greater"
//     branch when the controller has the resources but no chosen list.
func registerPhyrexianDreadnought(r *Registry) {
	r.OnETB("Phyrexian Dreadnought", phyrexianDreadnoughtETB)
}

func phyrexianDreadnoughtETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "phyrexian_dreadnought_etb_sac"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	totalPower := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		pw := gs.PowerOf(p)
		if pw > 0 {
			totalPower += pw
		}
	}
	if totalPower < 12 {
		gameengine.SacrificePermanent(gs, perm, slug)
		emit(gs, slug, "Phyrexian Dreadnought", map[string]interface{}{
			"seat":             perm.Controller,
			"sacrificed_self":  true,
			"available_power":  totalPower,
			"required_power":   12,
		})
		return
	}
	emit(gs, slug, "Phyrexian Dreadnought", map[string]interface{}{
		"seat":             perm.Controller,
		"sacrificed_self":  false,
		"available_power":  totalPower,
	})
	emitPartial(gs, slug, "Phyrexian Dreadnought",
		"sacrifice_chosen_subset_with_total_power_12_requires_ai_payment_decision_hook")
}
