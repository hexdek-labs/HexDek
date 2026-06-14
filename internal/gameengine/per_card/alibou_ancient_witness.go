package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlibouAncientWitness wires Alibou, Ancient Witness.
//
// Oracle text (Scryfall, verified 2026-05-04):
//
//	{3}{R}{W}
//	Legendary Artifact Creature — Golem
//	Other artifact creatures you control have haste.
//	Whenever one or more artifact creatures you control attack, Alibou
//	deals X damage to any target and you scry X, where X is the number
//	of tapped artifacts you control.
//
// Implementation:
//   - declare_attackers gated to controller: count tapped artifacts
//     controlled, deal X damage to highest-life opponent (or first
//     blocker if no opponents threaten lethal), scry X.
//   - "Other artifact creatures have haste" is a static layered effect
//     not modeled here — emitPartial.
func registerAlibouAncientWitness(r *Registry) {
	r.OnTrigger("Alibou, Ancient Witness", "declare_attackers", alibouAttacks)
}

func alibouAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "alibou_ancient_witness_ping_scry"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// "Whenever one or more artifact creatures you control attack" — a single
	// player-level attack-declaration event, fires EXACTLY ONCE per combat
	// regardless of how many artifact creatures attack. The engine dispatches
	// creature_attacks ONCE PER DECLARED ATTACKER with attacker_seat in ctx
	// (active_seat is never populated for attacks, so the old read returned 0:
	// inert for non-seat-0 controllers, and N×(damage+scry) for seat 0). Read
	// canonical attacker_seat + gate to the first hit per turn (Caesar/Raffine
	// once-per-turn dedup). r63 you-attack cardinality audit.
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["alibou_attack_fired"] >= gs.Turn {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	attackingArtifactCreature := false
	tappedArtifacts := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if cardHasType(p.Card, "artifact") {
			if p.Tapped {
				tappedArtifacts++
			}
			if p.IsCreature() && p.IsAttacking() {
				if def, ok := gameengine.AttackerDefender(p); ok && def != perm.Controller {
					attackingArtifactCreature = true
				}
			}
		}
	}
	if !attackingArtifactCreature || tappedArtifacts == 0 {
		return
	}
	// Trigger condition met — claim the once-per-turn slot only now, so a
	// combat with no attacking artifact creature doesn't consume it.
	perm.Flags["alibou_attack_fired"] = gs.Turn
	// Target highest-life opponent.
	target := -1
	bestLife := -1
	for i, opp := range gs.Seats {
		if opp == nil || opp.Lost || i == perm.Controller {
			continue
		}
		if opp.Life > bestLife {
			bestLife = opp.Life
			target = i
		}
	}
	if target >= 0 {
		gameengine.DealDamage(gs, target, tappedArtifacts, perm.Card.DisplayName())
	}
	gameengine.Scry(gs, perm.Controller, tappedArtifacts)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"tapped_artifacts": tappedArtifacts,
		"target_seat":      target,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"haste_grant_to_other_artifact_creatures_not_layered")
	_ = gs.CheckEnd()
}
