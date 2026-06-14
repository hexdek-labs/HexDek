package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMrFoxglove wires Mr. Foxglove.
//
// Oracle text:
//
//	Lifelink
//	Whenever Mr. Foxglove attacks, draw cards equal to the number of
//	cards in defending player's hand minus the number of cards in your
//	hand. If you didn't draw cards this way, you may put a creature
//	card from your hand onto the battlefield.
func registerMrFoxglove(r *Registry) {
	r.OnTrigger("Mr. Foxglove", "attacks", mrFoxgloveAttacks)
}

func mrFoxgloveAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mr_foxglove_attack_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever ~ attacks" fires only when this creature
	// itself is the attacker. The creature_attacks event is dispatched once per
	// declared attacker, so the old seat check (ctx["seat"] is never populated by
	// the attack fire → it only matched seat 0) let the effect multiply per
	// attacker. Mirrors the canonical gate and the Aang and Katara fix (5254f9cf).
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	defenderSeat := -1
	if d, ok := ctx["defender_seat"].(int); ok {
		defenderSeat = d
	}
	if defenderSeat < 0 {
		if def, ok := gameengine.AttackerDefender(perm); ok {
			defenderSeat = def
		}
	}
	mySeat := gs.Seats[perm.Controller]
	if mySeat == nil {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	defSeat := gs.Seats[defenderSeat]
	if defSeat == nil {
		return
	}
	diff := len(defSeat.Hand) - len(mySeat.Hand)
	if diff > 0 {
		drawn := 0
		for i := 0; i < diff && len(mySeat.Library) > 0; i++ {
			c := mySeat.Library[0]
			gameengine.MoveCard(gs, c, perm.Controller, "library", "hand", "draw")
			drawn++
		}
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":  perm.Controller,
			"draw":  drawn,
			"delta": diff,
		})
		return
	}
	for i, c := range mySeat.Hand {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		mySeat.Hand = append(mySeat.Hand[:i], mySeat.Hand[i+1:]...)
		enterBattlefieldWithETB(gs, perm.Controller, c, false)
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"draw":   0,
			"played": c.DisplayName(),
		})
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"draw": 0,
	})
}
