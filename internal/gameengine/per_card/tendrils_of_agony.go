package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTendrilsOfAgony wires Tendrils of Agony.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Tendrils%20of%20Agony):
//
//	Target opponent loses 2 life and you gain 2 life.
//	Storm  (When you cast this spell, copy it for each other spell cast
//	before it this turn. You may choose new targets for the copies.)
//
// {2}{B}{B} Sorcery. THE storm finisher — 9 prior casts means 10 total
// resolutions of "lose 2 / gain 2" = 20 life swing per opponent on the
// stack. In a 4-player cEDH game with one Tendrils as the finisher,
// the chain looks like Dark Ritual → Cabal Ritual → Mana Vault → Bolas's
// Citadel → cast-from-top chain → Tendrils with storm count 7-10, lethal.
//
// Implementation:
//   - OnResolve. The storm copies are pushed onto the stack BY THE
//     CAST PIPELINE (costs.go:843-844 calls ApplyStormCopies via
//     HasStormKeyword(card)), so by the time this OnResolve fires
//     for the ORIGINAL spell, the copies are already queued. Each
//     copy resolves separately and re-enters this OnResolve handler.
//   - Per-resolution effect: pick the highest-life opponent as the
//     "target opponent" (canonical hat policy when no explicit
//     target was stamped); they lose 2 life, controller gains 2.
//   - Routes through LoseLife + GainLife so the lifegain triggers
//     (Sanguine Bond, Marauding Blight-Priest patterns) and the
//     life-loss accumulator (Turn.LifeLost) bump per copy — exactly
//     what a real Tendrils chain needs.
func registerTendrilsOfAgony(r *Registry) {
	r.OnResolve("Tendrils of Agony", tendrilsOfAgonyResolve)
}

func tendrilsOfAgonyResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "tendrils_of_agony"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}

	// Pick the highest-life opponent.
	bestOpp := -1
	bestLife := -1
	for _, opp := range gs.Opponents(seat) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			bestOpp = opp
		}
	}
	if bestOpp < 0 {
		emitFail(gs, slug, "Tendrils of Agony", "no_valid_opponent", nil)
		return
	}

	gameengine.LoseLife(gs, bestOpp, 2, "Tendrils of Agony")
	gameengine.GainLife(gs, seat, 2, "Tendrils of Agony")

	emit(gs, slug, "Tendrils of Agony", map[string]interface{}{
		"seat":         seat,
		"target_seat":  bestOpp,
		"life_lost":    2,
		"life_gained":  2,
		"is_copy":      item.IsCopy,
	})
	_ = gs.CheckEnd()
}
