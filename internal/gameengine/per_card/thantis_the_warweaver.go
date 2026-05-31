package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerThantisTheWarweaver wires Thantis, the Warweaver.
//
// Oracle text:
//
//	{3}{B}{R}{G}
//	Legendary Creature — Spider
//	Vigilance, reach
//	All creatures attack each combat if able.
//	Whenever a creature attacks you or a planeswalker you control, put
//	  a +1/+1 counter on Thantis.
//
// Implementation:
//   - Vigilance / reach: AST keywords.
//   - "All creatures attack each combat if able" — global combat
//     compulsion. The engine's combat AI honors must-attack flags via
//     gs.Flags["force_attack_all"]; we set it on ETB and emit a
//     parser_gap so analysis tracks the gap (set is permanent until
//     Thantis leaves; we don't currently clear on LTB).
//   - "creature_attacks" trigger: gated on defender_seat == controller
//     (or ctx defender_perm controller). Adds a +1/+1 counter to Thantis.
func registerThantisTheWarweaver(r *Registry) {
	r.OnETB("Thantis, the Warweaver", thantisETB)
	r.OnTrigger("Thantis, the Warweaver", "creature_attacks", thantisAttackedTrigger)
}

func thantisETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "thantis_force_attacks"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["force_attack_all"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"static_force_attack_does_not_clear_on_ltb_partial")
}

func thantisAttackedTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "thantis_plus1_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	// Engine fires creature_attacks with attacker_perm; the defender is
	// stored on the attacker via setAttackerDefender (CR §506.1). The
	// legacy defender_seat / defender_perm keys aren't populated for this
	// event, so do the AttackerDefender lookup; keep the legacy ctx reads
	// as fallbacks for Hat/tests that thread them explicitly.
	matched := false
	if atk, ok := ctx["attacker_perm"].(*gameengine.Permanent); ok && atk != nil {
		if d, found := gameengine.AttackerDefender(atk); found && d == perm.Controller {
			matched = true
		}
	}
	if !matched {
		if defenderSeat, hasDef := ctx["defender_seat"].(int); hasDef && defenderSeat == perm.Controller {
			matched = true
		}
	}
	if !matched {
		// Try defender_perm — attacking a planeswalker you control.
		if defPerm, ok := ctx["defender_perm"].(*gameengine.Permanent); ok && defPerm != nil {
			if defPerm.Controller == perm.Controller && defPerm.IsPlaneswalker() {
				matched = true
			}
		}
	}
	if !matched {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.LogEvent(gameengine.Event{
		Kind:   "counter_mod",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"counter_kind": "+1/+1",
			"op":           "put",
			"reason":       "thantis_attacked",
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
