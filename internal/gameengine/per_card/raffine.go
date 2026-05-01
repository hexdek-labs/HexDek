package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRaffine wires Raffine, Scheming Seer.
//
// Oracle text:
//
//	Flying, ward {1}
//	Whenever you attack, target attacking creature connives X, where X
//	is the number of attacking creatures.
//
// "Whenever you attack" triggers once per declare-attackers step (CR
// §603 — single trigger keyed off the controller declaring at least
// one attacker, not once per attacker). The engine fires
// "creature_attacks" once per declared attacker, so we gate to the
// first hit per turn via a controller-level flag, mirroring the
// vialSmasherTrigger pattern.
func registerRaffine(r *Registry) {
	r.OnTrigger("Raffine, Scheming Seer", "creature_attacks", raffineTrigger)
}

func raffineTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "raffine_scheming_seer_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}

	flagKey := "raffine_fired"
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags[flagKey] >= gs.Turn {
		return
	}
	perm.Flags[flagKey] = gs.Turn

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	var attackers []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsAttacking() {
			continue
		}
		attackers = append(attackers, p)
	}
	count := len(attackers)
	if count == 0 {
		// Fall back to the ctx attacker if the flag scan missed (single
		// attacker case where flagAttacking is set just before this fires).
		if atk, ok := ctx["attacker_perm"].(*gameengine.Permanent); ok && atk != nil {
			attackers = []*gameengine.Permanent{atk}
			count = 1
		}
	}
	if count == 0 {
		return
	}

	// Pick best target: highest-power attacker (ties → Raffine herself
	// if she's attacking, else the newest creature).
	var target *gameengine.Permanent
	bestPower := -1
	for _, a := range attackers {
		pw := a.Power()
		if pw > bestPower {
			bestPower = pw
			target = a
		} else if pw == bestPower && a == perm {
			target = perm
		}
	}
	if target == nil {
		target = attackers[0]
	}

	gameengine.Connive(gs, target, count)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"attacker_count": count,
		"target":         target.Card.DisplayName(),
		"connive_n":      count,
	})
}
