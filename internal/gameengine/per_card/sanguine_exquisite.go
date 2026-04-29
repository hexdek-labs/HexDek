package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSanguineBond wires up Sanguine Bond.
//
// Oracle text:
//
//	Whenever you gain life, target opponent loses that much life.
//
// Implementation:
//   - OnTrigger "life_gained": if the gaining seat controls Sanguine
//     Bond, deal that much life loss to an opponent.
//   - Loop detection: if the engine detects more than 100 consecutive
//     life_gained triggers in one resolution, it stops (prevents
//     infinite loop with Exquisite Blood).
func registerSanguineBond(r *Registry) {
	r.OnTrigger("Sanguine Bond", "life_gained", sanguineBondTrigger)
}

func sanguineBondTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sanguine_bond_drain"
	if gs == nil || perm == nil {
		return
	}

	// Loop guard — prevent infinite Sanguine Bond + Exquisite Blood.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["sanguine_exquisite_loop"]++
	if gs.Flags["sanguine_exquisite_loop"] > 100 {
		// Infinite loop detected — all opponents lose.
		for _, opp := range gs.Opponents(perm.Controller) {
			if gs.Seats[opp] != nil && !gs.Seats[opp].Lost {
				gs.Seats[opp].Lost = true
				gs.Seats[opp].LossReason = "sanguine_bond_exquisite_blood_infinite_drain"
			}
		}
		gs.LogEvent(gameengine.Event{
			Kind:   "infinite_loop_detected",
			Seat:   perm.Controller,
			Source: "Sanguine Bond + Exquisite Blood",
			Details: map[string]interface{}{
				"iterations": gs.Flags["sanguine_exquisite_loop"],
				"resolution": "opponents_lose",
			},
		})
		return
	}

	// Only trigger if the life was gained by THIS permanent's controller.
	gainSeat := -1
	if v, ok := ctx["seat"].(int); ok {
		gainSeat = v
	}
	if gainSeat != perm.Controller {
		return
	}

	amount := 0
	if v, ok := ctx["amount"].(int); ok {
		amount = v
	}
	if amount <= 0 {
		return
	}

	// Target opponent loses that much life.
	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	// Pick the opponent with the most life (threat assessment).
	bestOpp := opps[0]
	bestLife := gs.Seats[opps[0]].Life
	for _, opp := range opps[1:] {
		if gs.Seats[opp].Life > bestLife {
			bestLife = gs.Seats[opp].Life
			bestOpp = opp
		}
	}

	gs.Seats[bestOpp].Life -= amount
	gs.LogEvent(gameengine.Event{
		Kind:   "life_change",
		Seat:   bestOpp,
		Source: "Sanguine Bond",
		Amount: -amount,
		Details: map[string]interface{}{
			"from": gs.Seats[bestOpp].Life + amount,
			"to":   gs.Seats[bestOpp].Life,
		},
	})

	emit(gs, slug, "Sanguine Bond", map[string]interface{}{
		"seat":     perm.Controller,
		"target":   bestOpp,
		"life_lost": amount,
	})

	// Fire life_gained event for Exquisite Blood interaction.
	// (Exquisite Blood triggers on opponent life loss -> you gain life)
	// This is handled by Exquisite Blood's own trigger below.
}

// registerExquisiteBlood wires up Exquisite Blood.
//
// Oracle text:
//
//	Whenever an opponent loses life, you gain that much life.
//
// Implementation:
//   - OnTrigger "life_lost_opponent": if opponent lost life and you
//     control Exquisite Blood, you gain that much life.
func registerExquisiteBlood(r *Registry) {
	r.OnTrigger("Exquisite Blood", "life_change", exquisiteBloodTrigger)
}

func exquisiteBloodTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "exquisite_blood_gain"
	if gs == nil || perm == nil {
		return
	}

	// Loop guard.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	if gs.Flags["sanguine_exquisite_loop"] > 100 {
		return
	}

	// Only trigger on life LOSS (negative amount) from an opponent.
	lossSeat := -1
	if v, ok := ctx["seat"].(int); ok {
		lossSeat = v
	}
	amount := 0
	if v, ok := ctx["amount"].(int); ok {
		amount = v
	}

	// amount is the change: negative = loss.
	if amount >= 0 {
		return // Not a loss
	}
	if lossSeat == perm.Controller {
		return // Not an opponent
	}

	gainAmount := -amount // Convert to positive
	gameengine.GainLife(gs, perm.Controller, gainAmount, "Exquisite Blood")

	emit(gs, slug, "Exquisite Blood", map[string]interface{}{
		"seat":      perm.Controller,
		"gained":    gainAmount,
		"from_loss": lossSeat,
	})
}
