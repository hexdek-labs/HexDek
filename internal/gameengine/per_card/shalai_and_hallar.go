package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerShalaiAndHallar wires Shalai and Hallar.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Flying, vigilance.
//	Whenever one or more +1/+1 counters are put on a creature you control,
//	Shalai and Hallar deals that much damage to each opponent.
//
// Implementation:
//   - Flying and vigilance are handled by the AST keyword pipeline.
//   - OnTrigger("counter_placed"): fires whenever resolveCounterMod places
//     counters on any permanent. We filter for:
//     (a) counter_kind == "+1/+1",
//     (b) target_seat == perm.Controller (a creature WE control),
//     (c) the target permanent is a creature.
//     Then deal `amount` damage to each living opponent by reducing Life.
//
// Note: the oracle reads "one or more +1/+1 counters are put on a creature
// you control" — it fires once per event with the total amount added, not
// once per counter. resolveCounterMod fires counter_placed once per target
// with the aggregate effectiveCount, which matches this wording correctly.
//
// Coverage gap: counter placements that bypass resolveCounterMod (combat
// infect, certain replacement effects) do not fire counter_placed and
// therefore will not trigger this handler. emitPartial flags the gap.
func registerShalaiAndHallar(r *Registry) {
	r.OnETB("Shalai and Hallar", shalaiAndHallarETB)
	r.OnTrigger("Shalai and Hallar", "counter_placed", shalaiAndHallarCounterPlaced)
}

func shalaiAndHallarETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "shalai_and_hallar_coverage_gap"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"counter_placed_not_fired_from_combat_infect_or_replacement_effects")
}

func shalaiAndHallarCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "shalai_and_hallar_damage_each_opponent"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Only trigger on +1/+1 counters.
	kind, _ := ctx["counter_kind"].(string)
	if kind != "+1/+1" {
		return
	}

	// Only trigger when the counter went on a creature WE control.
	targetSeat, _ := ctx["target_seat"].(int)
	if targetSeat != perm.Controller {
		return
	}
	targetPerm, _ := ctx["target_perm"].(*gameengine.Permanent)
	if targetPerm == nil || !targetPerm.IsCreature() {
		return
	}

	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}

	// Deal `amount` damage to each living opponent.
	opps := gs.Opponents(perm.Controller)
	for _, opp := range opps {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		s.Life -= amount
		gs.LogEvent(gameengine.Event{
			Kind:   "damage",
			Seat:   opp,
			Target: opp,
			Source: perm.Card.DisplayName(),
			Amount: amount,
			Details: map[string]interface{}{
				"reason":       "shalai_and_hallar_counter_trigger",
				"counter_kind": kind,
				"on_creature":  targetPerm.Card.DisplayName(),
			},
		})
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"target_card":   targetPerm.Card.DisplayName(),
		"counter_kind":  kind,
		"amount":        amount,
		"opponents_hit": len(opps),
	})

	_ = gs.CheckEnd()
}
