package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarlachFuryOfAvernus wires Karlach, Fury of Avernus.
// Batch #33.
//
// Oracle text (Commander Legends: Battle for Baldur's Gate, {4}{R},
// Legendary Creature — Tiefling Barbarian, 5/4):
//
//	Whenever you attack, if it's the first combat phase of the turn,
//	untap all attacking creatures. They gain first strike until end
//	of turn. After this phase, there is an additional combat phase.
//	Choose a Background (You can have a Background as a second
//	commander.)
//
// Implementation:
//   - "Choose a Background" is partner-shaped metadata handled at the
//     deck/commander layer; the in-game ability is a no-op for the
//     handler.
//   - "combat_begin" trigger: stamp a turn-keyed counter on Karlach's
//     Flags so the attacks trigger can detect "first combat phase".
//     Mirrors the Tifa, Martial Artist pattern.
//   - "Whenever you attack" → gate "creature_attacks" on
//     attacker_seat == perm.Controller, then dedupe per (turn, combat-
//     idx) so Karlach fires once per declare-attackers step regardless
//     of how many attackers her controller declared (CR §603 — single
//     trigger keyed off the player declaring at least one attacker).
//     Gate further on combat_idx == 1 (first combat phase). On fire:
//     untap each attacking creature Karlach's controller controls,
//     grant first strike until end of turn (delayed trigger clears the
//     keyword flag at next end step), and bump
//     gs.PendingExtraCombats so the turn loop runs another combat.
func registerKarlachFuryOfAvernus(r *Registry) {
	r.OnTrigger("Karlach, Fury of Avernus", "combat_begin", karlachCombatBegin)
	r.OnTrigger("Karlach, Fury of Avernus", "creature_attacks", karlachAttacks)
}

func karlachCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := karlachTurnKey(gs.Turn)
	if perm.Flags["karlach_turn_marker"] != turnKey {
		perm.Flags["karlach_turn_marker"] = turnKey
		perm.Flags["karlach_combat_idx"] = 0
		perm.Flags["karlach_fired_combat_idx"] = 0
	}
	perm.Flags["karlach_combat_idx"]++
}

func karlachAttacks(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "karlach_fury_of_avernus_first_combat_extra"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	attackerSeat, _ := ctx["attacker_seat"].(int)
	if attackerSeat != perm.Controller {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	turnKey := karlachTurnKey(gs.Turn)
	combatIdx := perm.Flags["karlach_combat_idx"]
	if combatIdx == 0 {
		// combat_begin didn't seed the counter (older turn paths or
		// direct test entry) — assume we're in the first combat.
		combatIdx = 1
		perm.Flags["karlach_turn_marker"] = turnKey
		perm.Flags["karlach_combat_idx"] = 1
	}
	if combatIdx != 1 {
		return
	}
	// Dedupe — fire once per (turn, combat) regardless of attacker count.
	if perm.Flags["karlach_fired_turn_marker"] == turnKey &&
		perm.Flags["karlach_fired_combat_idx"] == combatIdx {
		return
	}
	perm.Flags["karlach_fired_turn_marker"] = turnKey
	perm.Flags["karlach_fired_combat_idx"] = combatIdx

	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	untapped := 0
	granted := 0
	var firstStrikeGrants []*gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() || !p.IsAttacking() {
			continue
		}
		if p.Tapped {
			p.Tapped = false
			untapped++
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		if p.Flags["kw:first strike"] == 0 {
			p.Flags["kw:first strike"] = 1
			firstStrikeGrants = append(firstStrikeGrants, p)
			granted++
		}
	}

	if len(firstStrikeGrants) > 0 {
		grants := firstStrikeGrants
		gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
			TriggerAt:      "next_end_step",
			ControllerSeat: perm.Controller,
			SourceCardName: perm.Card.DisplayName(),
			OneShot:        true,
			EffectFn: func(gs *gameengine.GameState) {
				for _, p := range grants {
					if p == nil || p.Flags == nil {
						continue
					}
					delete(p.Flags, "kw:first strike")
				}
			},
		})
	}

	gs.PendingExtraCombats++
	gs.LogEvent(gameengine.Event{
		Kind:   "extra_combat",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason": "karlach_first_combat",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"untapped":         untapped,
		"first_strike_grants": granted,
		"combat_idx":       combatIdx,
		"pending_combats":  gs.PendingExtraCombats,
	})
}

func karlachTurnKey(turn int) int {
	return turn + 1
}
