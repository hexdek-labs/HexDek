package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSepticRats wires Septic Rats (Muninn parser-gap #49,
// 16,222 hits).
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	{1}{B}{B}
//	Creature — Phyrexian Rat
//	Infect
//	Whenever this creature attacks, if defending player is poisoned,
//	it gets +1/+1 until end of turn.
//
// Implementation:
//   - Infect is engine-side via the keyword pipeline; no handler needed.
//   - Attack trigger: OnTrigger("creature_attacks"), gate on
//     attacker_perm == perm. Look up defender_seat in ctx and check
//     PoisonCounters > 0. If so, pump +1/+1 until end of turn through
//     pumpUntilEOT (pt_pump.go). NOTE: this used to write only
//     Flags["temp_power"] / ["temp_toughness"], described here as "the
//     +1/+1-until-EOT convention used across per_card". There was no
//     such convention — nothing in the engine reads those flags, so the
//     pump did nothing.
//   - If defender_seat is missing (planeswalker attack target, etc.),
//     fall back to "any opponent is poisoned" — the trigger reads
//     "defending player is poisoned" but ctx may not route the seat for
//     non-player attack targets. Single-poisoned-opponent fallback is a
//     conservative approximation; emitPartial flags it.
func registerSepticRats(r *Registry) {
	// "creature_attacks" is the canonical engine event for combat
	// attacks; the "attacks" alias aliases to the same normalized event,
	// so registering only one prevents double-fire.
	r.OnTrigger("Septic Rats", "creature_attacks", septicRatsAttacks)
}

func septicRatsAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "septic_rats_attack_pump"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}

	dseat, hasDef := ctx["defender_seat"].(int)
	poisoned := false
	switch {
	case hasDef && dseat >= 0 && dseat < len(gs.Seats) && gs.Seats[dseat] != nil:
		poisoned = gs.Seats[dseat].PoisonCounters > 0
	default:
		// Fallback: any living opponent has poison.
		for i, s := range gs.Seats {
			if s == nil || s.Lost || i == perm.Controller {
				continue
			}
			if s.PoisonCounters > 0 {
				poisoned = true
				break
			}
		}
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"defender_seat_missing_fallback_any_poisoned_opp")
	}

	if !poisoned {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      perm.Controller,
			"defender":  dseat,
			"triggered": false,
		})
		return
	}

	pumpUntilEOT(gs, perm, 1, 1)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"defender":  dseat,
		"triggered": true,
	})
}
