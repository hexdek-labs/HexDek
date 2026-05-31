package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAloySaviorOfMeridian wires Aloy, Savior of Meridian.
//
// Oracle text:
//
//	Vigilance, reach
//	In You, All Things Are Possible — Whenever one or more artifact
//	creatures you control attack, discover X, where X is the greatest
//	power among them.
//
// Implementation:
//   - Vigilance + reach: handled by the AST keyword pipeline.
//   - "creature_attacks" trigger fires once per attack declaration.
//     Because the printed trigger is "one or more", we collapse
//     multiple attack-declarations in the same combat into a single
//     trigger fire via a once-per-turn flag, and compute X as the
//     greatest power among ALL attacking artifact creatures we
//     control at the moment the trigger fires.
//
// R60 stub sweep batch 6: dropped the stale "discover X not yet wired
// through per_card" doc claim — the handler does call
// gameengine.ApplyDiscover with the computed X, which wraps the full
// §702.165 mechanic (exile-until-nonland-of-≤cost, cast-or-hand,
// shuffle rest random to bottom). Emit the X value alongside.
func registerAloySaviorOfMeridian(r *Registry) {
	r.OnTrigger("Aloy, Savior of Meridian", "creature_attacks", aloyAttacks)
}

func aloyAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aloy_artifact_creatures_attack_discover"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil || atk.Controller != perm.Controller {
		return
	}
	if !atk.IsCreature() || atk.Card == nil || !cardHasType(atk.Card, "artifact") {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["aloy_fired_turn"] == gs.Turn {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	maxPow := 0
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p.Card == nil || !cardHasType(p.Card, "artifact") {
			continue
		}
		if !p.IsAttacking() {
			continue
		}
		count++
		if pow := gs.PowerOf(p); pow > maxPow {
			maxPow = pow
		}
	}
	perm.Flags["aloy_fired_turn"] = gs.Turn
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"attacker_count": count,
		"discover_x":     maxPow,
	})
	if maxPow > 0 {
		// CR §702.165 — discover X: exile cards from the top of the
		// library until you exile a nonland card with MV ≤ X. You may
		// cast it without paying its mana cost or put it into your
		// hand. ApplyDiscover wraps the full mechanic.
		gameengine.ApplyDiscover(gs, perm.Controller, maxPow)
	}
}
