package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTerrorOfThePeaks — non-keyword cost-on-target test card for
// the r60 unified WardCost primitive's Damage variant. Per the
// 7174n1c architecture lock (2026-05-27): the WardCost dispatcher
// must cover structurally similar triggered damage-on-target effects
// that aren't templated as the Ward keyword, so this handler routes
// through ApplyWardCostDamage instead of inlining its own damage
// resolution.
//
// Printed oracle (Scryfall, verified 2026-05-27):
//
//	Flying
//	Whenever another creature enters under your control, Terror of the
//	Peaks deals damage equal to that creature's power to any target.
//
// The "any target" can be an opponent player OR an opponent's creature
// (or your own, though the EV-positive choice is always opponent).
// ApplyWardCostDamage handles the damage routing through the standard
// FireDamageEvent + PreventDamageToPermanent / DealDamage(player)
// pipeline so replacement effects + prevention shields fire normally.
//
// Target selection: lowest-toughness opposing creature first (kills
// most efficiently); falls back to the lowest-life opponent if no
// creatures can be killed. Heuristic mirrors the standard ETB-ping
// target priority used by Soul-Scar Mage / Cunning Sparkmage handlers.
func registerTerrorOfThePeaks(r *Registry) {
	// Uses the same nonland_permanent_etb fan-out The Great Henge consumes —
	// ctx provides {perm, controller_seat, card}. Filter to creatures
	// inside the handler.
	r.OnTrigger("Terror of the Peaks", "nonland_permanent_etb", terrorOfThePeaksCreatureETB)
}

func terrorOfThePeaksCreatureETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "terror_of_the_peaks_etb_damage"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	// "another creature" — exclude the Terror itself.
	if entering == perm {
		return
	}
	if !entering.IsCreature() {
		return
	}
	// Must enter under perm's controller.
	if entering.Controller != perm.Controller {
		return
	}
	enteredCardName := entering.Card.DisplayName()
	enteredPower := entering.Card.BasePower
	if enteredPower <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":          perm.Controller,
			"entered_card":  enteredCardName,
			"entered_power": enteredPower,
			"result":        "no_damage_zero_power",
		})
		return
	}

	// Pick lowest-toughness opposing creature as the canonical target.
	var targetPerm *gameengine.Permanent
	bestT := 1 << 30
	for i, seat := range gs.Seats {
		if i == perm.Controller || seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			tough := p.Card.BaseToughness - p.MarkedDamage
			if tough < bestT {
				bestT = tough
				targetPerm = p
			}
		}
	}
	// Fall back to lowest-life opponent if no creature targets exist.
	targetSeat := -1
	if targetPerm == nil {
		bestLife := 1 << 30
		for i, seat := range gs.Seats {
			if i == perm.Controller || seat == nil || seat.Lost {
				continue
			}
			if seat.Life < bestLife {
				bestLife = seat.Life
				targetSeat = i
			}
		}
	}

	cost := gameengine.WardCost{
		Type:   gameengine.WardCostDamage,
		Amount: enteredPower,
	}
	dealt, detail := gameengine.ApplyWardCostDamage(gs, perm, targetPerm, targetSeat, cost)
	base := map[string]interface{}{
		"seat":         perm.Controller,
		"entered_card": enteredCardName,
		"dealt":        dealt,
	}
	for k, v := range detail {
		base[k] = v
	}
	emit(gs, slug, perm.Card.DisplayName(), base)
}
