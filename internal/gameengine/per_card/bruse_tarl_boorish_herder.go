package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBruseTarlBoorishHerder wires Bruse Tarl, Boorish Herder.
//
// Oracle text:
//
//	Whenever Bruse Tarl enters or attacks, target creature you control
//	gains double strike and lifelink until end of turn.
//	Partner
func registerBruseTarlBoorishHerder(r *Registry) {
	r.OnETB("Bruse Tarl, Boorish Herder", bruseTarlBoost)
	r.OnTrigger("Bruse Tarl, Boorish Herder", "attacks", bruseTarlAttacks)
}

func bruseTarlBoost(gs *gameengine.GameState, perm *gameengine.Permanent) {
	bruseTarlGrant(gs, perm, "etb")
}

func bruseTarlAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
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
	bruseTarlGrant(gs, perm, "attack")
}

func bruseTarlGrant(gs *gameengine.GameState, perm *gameengine.Permanent, cause string) {
	const slug = "bruse_tarl_grant"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	var best *gameengine.Permanent
	bestPow := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pow := p.Power(); pow > bestPow {
			best = p
			bestPow = pow
		}
	}
	if best == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_creatures", map[string]interface{}{"cause": cause})
		return
	}
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["kw:double_strike"] = 1
	best.Flags["kw:lifelink"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
		"cause":  cause,
	})
}
