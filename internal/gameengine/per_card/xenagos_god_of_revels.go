package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerXenagosGodOfRevels wires Xenagos, God of Revels.
//
// Oracle text:
//
//	Indestructible
//	As long as your devotion to red and green is less than seven, Xenagos
//	isn't a creature.
//	At the beginning of combat on your turn, another target creature you
//	control gains haste and gets +X/+X until end of turn, where X is that
//	creature's power.
//
// Implementation:
//   - "combat_begin" on Xenagos's controller's turn: pick the highest-
//     power other creature we control, grant haste + double its power.
//   - Indestructible and devotion-creature-toggle are static; emitPartial
//     for the devotion gate (the perm only fires triggers when on
//     battlefield as a creature, so this is mostly defensive).
func registerXenagosGodOfRevels(r *Registry) {
	r.OnTrigger("Xenagos, God of Revels", "combat_begin", xenagosCombatBegin)
}

func xenagosCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "xenagos_god_of_revels_haste_double"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Devotion gate: must be 7+ across R+G for Xenagos to be a creature
	// and trigger. Enforce defensively.
	if dev := countDevotion(seat, "R", "G"); dev < 7 {
		emitFail(gs, slug, perm.Card.DisplayName(), "devotion_below_seven", map[string]interface{}{
			"devotion": dev,
		})
		return
	}

	var best *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == perm || !p.IsCreature() {
			continue
		}
		if best == nil || p.Power() > best.Power() {
			best = p
		}
	}
	if best == nil {
		return
	}

	x := best.Power()
	if x <= 0 {
		x = 1
	}
	best.Modifications = append(best.Modifications, gameengine.Modification{
		Power:     x,
		Toughness: x,
		Duration:  "until_end_of_turn",
	})
	if best.Flags == nil {
		best.Flags = map[string]int{}
	}
	best.Flags["kw:haste"] = 1
	captured := best
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			gameengine.ClearKeywordFlags(gs, captured, "haste")
		},
	})
	gs.InvalidateCharacteristicsCache()

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": best.Card.DisplayName(),
		"x":      x,
	})
}
