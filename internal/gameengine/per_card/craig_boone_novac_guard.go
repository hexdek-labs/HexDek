package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerCraigBooneNovacGuard wires Craig Boone, Novac Guard.
//
// Oracle text:
//
//	Reach, lifelink
//	One for My Baby — Whenever you attack with two or more creatures,
//	put two quest counters on Craig Boone. When you do, Craig Boone
//	deals damage equal to the number of quest counters on it to up to
//	one target creature unless that creature's controller has Craig
//	Boone deal that much damage to them.
//
// Implementation: track "attack_with_count >= 2" via the engine's
// declare_attackers event ctx.
func registerCraigBooneNovacGuard(r *Registry) {
	r.OnTrigger("Craig Boone, Novac Guard", "declare_attackers", craigBooneAttack)
}

func craigBooneAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "craig_boone_quest_counters"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires creature_attacks (canonical = "attack" after alias
	// normalization) with attacker_perm + attacker_seat + attacker_card —
	// no "count" key, since the trigger fires once per attacker. Legacy
	// "seat" + "count" reads kept as fallbacks for callers that batch them.
	attackerSeat, ok := ctx["attacker_seat"].(int)
	if !ok {
		attackerSeat, _ = ctx["seat"].(int)
	}
	if attackerSeat != perm.Controller {
		return
	}
	// "Whenever you attack with two or more creatures" — once per combat
	// per controller. Gate via a per-turn flag stamped with gs.Turn+1
	// (the +1 keeps it distinct from default zero on turn 0). Count
	// attackers by scanning the controller's battlefield rather than
	// relying on a non-existent ctx["count"].
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["craig_boone_fired_turn"] == gs.Turn+1 {
		return
	}
	count := 0
	if seat := gs.Seats[perm.Controller]; seat != nil {
		for _, p := range seat.Battlefield {
			if p != nil && p.IsCreature() && p.IsAttacking() {
				count++
			}
		}
	}
	if count < 2 {
		// Fallback to ctx-supplied count if a batched caller threaded it
		// (compat with synthetic fire-sites that pre-aggregate the count).
		if v, ok := ctx["count"].(int); ok && v >= 2 {
			count = v
		} else {
			return
		}
	}
	perm.Flags["craig_boone_fired_turn"] = gs.Turn + 1
	perm.AddCounter("quest", 2)
	dmg := perm.Counters["quest"]
	// Damage to best opposing creature; planar choice = creature.
	var best *gameengine.Permanent
	bestPow := -1
	for i, opp := range gs.Seats {
		if opp == nil || i == perm.Controller || opp.Lost {
			continue
		}
		for _, p := range opp.Battlefield {
			if p == nil || !p.IsCreature() {
				continue
			}
			if pow := p.Power(); pow > bestPow {
				best = p
				bestPow = pow
			}
		}
	}
	target := ""
	if best != nil {
		gameengine.FireDamageEvent(gs, perm, best.Controller, best, dmg)
		target = best.Card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"counters": perm.Counters["quest"],
		"damage":   dmg,
		"target":   target,
	})
}
