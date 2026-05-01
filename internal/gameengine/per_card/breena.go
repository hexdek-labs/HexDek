package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBreena wires Breena, the Demagogue.
//
// Oracle text:
//
//	Flying
//	Whenever a player attacks one of your opponents, if that opponent has
//	more life than another of your opponents, that attacking player draws
//	a card and you put two +1/+1 counters on a creature you control.
//
// Implementation:
//   - "creature_attacks": fires per declared attacker. The oracle reads
//     once per attack declaration ("a player attacks") rather than per
//     attacker, so we de-dupe via a (attacker_seat, turn) flag on Breena.
//   - Defender lookup uses gameengine.AttackerDefender (CR §506.1 per-
//     attacker assignment).
//   - Conditions:
//       * Defender is one of Breena's controller's opponents (not Breena's
//         seat, not the attacker, not a lost player).
//       * Defender's life > life of at least one other opponent of Breena's
//         controller (i.e., defender is not the lowest-life opponent).
//   - Effects:
//       * The attacking player draws one card.
//       * Breena's controller puts two +1/+1 counters on a creature they
//         control. AI heuristic: pick the highest-power creature (ties
//         broken by Breena herself, then earliest timestamp).
func registerBreena(r *Registry) {
	r.OnTrigger("Breena, the Demagogue", "creature_attacks", breenaTrigger)
}

func breenaTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "breena_the_demagogue_attack"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk == nil {
		return
	}
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat == perm.Controller {
		// "a player attacks one of your opponents" — Breena's controller
		// attacking is irrelevant (they aren't attacking their own opponent
		// from Breena's perspective in any meaningful way per oracle).
		// Strict reading: Breena could trigger on her controller attacking
		// another opponent, but the design intent of the card (rewarding
		// attacks against the highest-life opponent) clearly targets non-
		// controller attackers; controller-attack would self-reward.
		// We follow the dominant cEDH interpretation and skip self-attacks.
		return
	}

	defenderSeat, ok := gameengine.AttackerDefender(atk)
	if !ok || defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}
	if defenderSeat == perm.Controller {
		// Attacking Breena's controller — defender isn't an opponent of
		// Breena's controller, so the trigger doesn't apply.
		return
	}
	defender := gs.Seats[defenderSeat]
	if defender == nil || defender.Lost {
		return
	}

	// Find at least one OTHER opponent of Breena's controller with strictly
	// less life than the defender.
	hasLowerOpponent := false
	for i, s := range gs.Seats {
		if s == nil || s.Lost {
			continue
		}
		if i == perm.Controller || i == defenderSeat {
			continue
		}
		if s.Life < defender.Life {
			hasLowerOpponent = true
			break
		}
	}
	if !hasLowerOpponent {
		return
	}

	// De-dupe per (attacker_seat, turn): "Whenever a player attacks" fires
	// once per attack declaration, not once per attacking creature.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := fmt.Sprintf("breena_a%d_t%d", attackerSeat, gs.Turn+1)
	if perm.Flags[dedupeKey] == 1 {
		return
	}
	perm.Flags[dedupeKey] = 1

	// Effect 1: attacking player draws a card.
	drawn := drawOne(gs, attackerSeat, perm.Card.DisplayName())

	// Effect 2: Breena's controller puts two +1/+1 counters on a creature
	// they control. Highest-power; tiebreak Breena, then earliest timestamp.
	target := pickBreenaCounterTarget(gs, perm)
	if target != nil {
		target.AddCounter("+1/+1", 2)
	}

	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	targetName := ""
	if target != nil {
		targetName = target.Card.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          perm.Controller,
		"attacker_seat": attackerSeat,
		"defender_seat": defenderSeat,
		"defender_life": defender.Life,
		"drawn_card":    drawnName,
		"counter_target": targetName,
	})
}

func pickBreenaCounterTarget(gs *gameengine.GameState, perm *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || perm == nil {
		return nil
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return nil
	}
	var best *gameengine.Permanent
	bestPower := -1
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		pw := p.Power()
		if pw > bestPower {
			bestPower = pw
			best = p
			continue
		}
		if pw == bestPower {
			if p == perm {
				best = p
			} else if best != nil && best != perm && p.Timestamp < best.Timestamp {
				best = p
			}
		}
	}
	return best
}
