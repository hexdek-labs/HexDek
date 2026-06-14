package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOmoQueenOfVesuva wires Omo, Queen of Vesuva.
//
// Oracle text:
//
//	Whenever Omo enters or attacks, put an everything counter on each
//	of up to one target land and up to one target creature.
//	Each land with an everything counter on it is every land type in
//	addition to its other types.
//	Each nonland creature with an everything counter on it is every
//	creature type.
//
// "Everything counter" type-grant is a static effect class the engine
// doesn't model; we stamp counters but flag the type-grant as partial.
func registerOmoQueenOfVesuva(r *Registry) {
	r.OnETB("Omo, Queen of Vesuva", omoETB)
	r.OnTrigger("Omo, Queen of Vesuva", "attacks", omoAttack)
}

func omoStampCounters(gs *gameengine.GameState, perm *gameengine.Permanent, slug, source string) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick the most impactful land: prefer one that doesn't already have
	// the everything counter (no point doubling up), and otherwise the
	// first one we see. Skip lands that are already every type by virtue
	// of having the counter.
	var land *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsLand() {
			continue
		}
		if p.Counters["everything"] > 0 {
			continue
		}
		land = p
		break
	}
	// Pick the highest-power non-Omo creature; everything-counter on a
	// big body is more useful (more tribal interactions, more relevant
	// type lines for buffs). Falls back to Omo herself if no other
	// creature is available.
	var creature *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || p == perm {
			continue
		}
		if p.Counters["everything"] > 0 {
			continue
		}
		pw := p.Power()
		if pw > bestPower {
			bestPower = pw
			creature = p
		}
	}
	if creature == nil && perm.IsCreature() && perm.Counters["everything"] == 0 {
		creature = perm
	}
	if land != nil {
		land.AddCounter("everything", 1)
	}
	if creature != nil {
		creature.AddCounter("everything", 1)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"source":   source,
		"land":     land != nil,
		"creature": creature != nil,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(), "everything_counter_type_grant_unimplemented")
}

func omoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	omoStampCounters(gs, perm, "omo_etb_everything", "etb")
}

func omoAttack(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// CR §603.2 self-gate: "Whenever Omo enters or attacks" — on the attack
	// branch, fire only when Omo itself is the attacker. creature_attacks is
	// dispatched once per declared attacker, so the old seat check (ctx["seat"]
	// is never populated → matched only seat 0) stamped counters once per
	// attacker. See the Aang and Katara fix (5254f9cf). ETB dispatch passes no
	// attacker_perm, so this branch is reached only via the attack fire.
	if atk, _ := ctx["attacker_perm"].(*gameengine.Permanent); atk != perm {
		return
	}
	omoStampCounters(gs, perm, "omo_attack_everything", "attack")
}
