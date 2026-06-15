package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerGrevenPredatorCaptain wires Greven, Predator Captain.
//
// Oracle text:
//
//	Menace
//	Greven gets +X/+0, where X is the amount of life you've lost
//	this turn.
//	Whenever Greven attacks, you may sacrifice another creature.
//	If you do, you draw cards equal to that creature's power and
//	you lose life equal to that creature's toughness.
//
// Implementation:
//   - attacks trigger: sac best power/toughness-ratio creature, draw
//     power cards, lose toughness life.
//   - life_lost trigger: applies incremental +X/+0 Modification
//     tracking seat.Turn.LifeLost via perm.Flags["greven_pump"].
func registerGrevenPredatorCaptain(r *Registry) {
	r.OnTrigger("Greven, Predator Captain", "attacks", grevenAttacks)
	r.OnTrigger("Greven, Predator Captain", "life_lost", grevenPumpUpdate)
}

func grevenPumpUpdate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	lossSeat, _ := ctx["seat"].(int)
	if lossSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	pump := seat.Turn.LifeLost
	alreadyApplied := 0
	if perm.Flags != nil {
		alreadyApplied = perm.Flags["greven_pump"]
	}
	delta := pump - alreadyApplied
	if delta > 0 {
		perm.Modifications = append(perm.Modifications, gameengine.Modification{
			Power:    delta,
			Duration: "until_end_of_turn",
		})
		if perm.Flags == nil {
			perm.Flags = map[string]int{}
		}
		perm.Flags["greven_pump"] = pump
	}
}

func grevenAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "greven_attacks_sac_draw"
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
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	var pick *gameengine.Permanent
	bestScore := -1
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "creature") {
			continue
		}
		power := int(p.Card.BasePower)
		tough := int(p.Card.BaseToughness)
		if seat.Life-tough <= 0 {
			continue
		}
		score := power*10 - tough
		if score > bestScore {
			bestScore = score
			pick = p
		}
	}
	if pick == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"sacrificed": false,
		})
		return
	}
	// CR §603.10 / §608.2g — "draw cards equal to that creature's power
	// and you lose life equal to its toughness" reads the sacrificed
	// creature's LAST-KNOWN P/T: its effective power/toughness including
	// +1/+1 counters, anthems, and other continuous effects, NOT its
	// printed base. Read via the layer-aware PowerOf/ToughnessOf while the
	// creature is still on the battlefield (i.e. immediately before it's
	// sacrificed below). Reading Card.BasePower/BaseToughness ignored every
	// boost — a +1/+1-countered or anthem-buffed sacrifice drew/lost the
	// base amount.
	power := gs.PowerOf(pick)
	tough := gs.ToughnessOf(pick)
	if power < 0 {
		power = 0
	}
	if tough < 0 {
		tough = 0
	}
	gameengine.SacrificePermanent(gs, pick, "greven_predator_captain")
	for i := 0; i < power && len(seat.Library) > 0; i++ {
		card := seat.Library[0]
		gameengine.MoveCard(gs, card, perm.Controller, "library", "hand", "greven-draw")
	}
	gameengine.LoseLife(gs, perm.Controller, tough, perm.Card.DisplayName())
	_ = gs.CheckEnd()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"sacrificed": true,
		"power":      power,
		"toughness":  tough,
		"pump":       seat.Turn.LifeLost,
	})
}
