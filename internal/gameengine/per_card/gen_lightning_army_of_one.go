package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLightningArmyOfOne wires Lightning, Army of One.
//
// Oracle text:
//
//	First strike, trample, lifelink
//	Stagger — Whenever Lightning deals combat damage to a player,
//	until your next turn, if a source would deal damage to that
//	player or a permanent that player controls, it deals double that
//	damage instead.
//
// Implementation:
//   - First strike / trample / lifelink: handled by the AST keyword
//     pipeline.
//   - "combat_damage_to_player" trigger gated to attacker == Lightning:
//     arm a "stagger" flag on gs.Flags keyed by the damaged player's
//     seat and Lightning's controller's NEXT turn (current turn + N
//     where N is the seat count, approximating "until your next turn").
//
// emitPartial: the actual damage-doubling replacement effect needs an
// engine-side replacement-effect framework that consults the staged
// flag — we fire the arm event so downstream observers can pick it up
// when that wiring lands.
func registerLightningArmyOfOne(r *Registry) {
	r.OnTrigger("Lightning, Army of One", "combat_damage_to_player", lightningStaggerArm)
	// R51 batch I: defensive LTB clear of all lightning_stagger_seat*
	// _until_turn keys this Lightning armed. Even though the stagger
	// flag is self-expiring via turn comparison, downstream scanners
	// that walk the flag map keep paying the cost of a stale key for
	// every damage check between LTB and the natural expiry tick.
	r.OnTrigger("Lightning, Army of One", "permanent_ltb", lightningLTBClearStagger)
}

func lightningLTBClearStagger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	if gs.Flags == nil {
		return
	}
	const prefix = "lightning_stagger_seat"
	for k := range gs.Flags {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(gs.Flags, k)
		}
	}
}

func lightningStaggerArm(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lightning_stagger_arm"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	atk, _ := ctx["attacker_perm"].(*gameengine.Permanent)
	if atk != perm {
		return
	}
	defender := -1
	if v, ok := ctx["defender"].(int); ok {
		defender = v
	} else if v, ok := ctx["defender_seat"].(int); ok {
		defender = v
	} else if v, ok := ctx["target_seat"].(int); ok {
		defender = v
	}
	if defender < 0 || defender >= len(gs.Seats) {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	// Approximate "until your next turn" as current turn + len(seats),
	// since each seat takes one turn before the controller's next turn.
	expiresOnTurn := gs.Turn + len(gs.Seats)
	key := fmt.Sprintf("lightning_stagger_seat%d_until_turn", defender)
	gs.Flags[key] = expiresOnTurn
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"defender_seat":  defender,
		"expires_turn":   expiresOnTurn,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"damage_doubling_replacement_effect_not_wired_engine_side_TODO")
}
