package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSamutTheDrivingForce wires Samut, the Driving Force.
//
// Oracle text:
//
//	First strike, vigilance, haste
//	Start your engines! (If you have no speed, it starts at 1. It
//	increases once on each of your turns when an opponent loses life.
//	Max speed is 4.)
//	Other creatures you control get +X/+0, where X is your speed.
//	Noncreature spells you cast cost {X} less to cast, where X is
//	your speed.
//
// Implementation:
//   - Speed mechanic: per-seat counter tracked at seat.Flags["speed"].
//     Bumps once per turn (gated by seat.Flags["speed_bumped_this_turn"])
//     when an opponent loses life. Reset gate at upkeep_controller.
//   - +X/+0 anthem: refresh on permanent_etb / opponent_loses_life so
//     the buff tracks the current speed value.
//   - Noncreature cost reduction: engine-deep cost-modifier hook;
//     partial breadcrumb.
func registerSamutTheDrivingForce(r *Registry) {
	r.OnETB("Samut, the Driving Force", samutETBInitSpeed)
	r.OnTrigger("Samut, the Driving Force", "life_lost", samutOnOpponentLoseLife)
	r.OnTrigger("Samut, the Driving Force", "upkeep_controller", samutClearTurnGate)
	// R51 batch I: LTB clears the per-turn speed-bump gate so a Samut
	// that left mid-turn doesn't leave the gate stuck for the rest of
	// the turn. The "speed" counter itself is intentionally preserved
	// — speed is a player property (CR start-your-engines) that
	// persists past the source's leaving.
	r.OnTrigger("Samut, the Driving Force", "permanent_ltb", samutLTBClearTurnGate)
}

func samutLTBClearTurnGate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	delete(seat.Flags, "speed_bumped_this_turn")
}

func samutETBInitSpeed(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "samut_driving_force_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	if seat.Flags["speed"] < 1 {
		seat.Flags["speed"] = 1
	}
	samutRegisterAnthemLayer(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"speed": seat.Flags["speed"],
	})
	// Noncreature {X}-discount is wired in gameengine/cost_modifiers.go
	// (R50 batchH — "Samut, the Driving Force" case reads
	// seat.Flags["speed"]).
}

func samutOnOpponentLoseLife(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	loseSeat, _ := ctx["seat"].(int)
	if loseSeat == perm.Controller {
		return // own life loss doesn't trigger
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		if seat != nil {
			seat.Flags = map[string]int{}
		}
	}
	if seat == nil {
		return
	}
	if seat.Flags["speed_bumped_this_turn"] == 1 {
		return
	}
	if seat.Flags["speed"] >= 4 {
		return
	}
	seat.Flags["speed"]++
	seat.Flags["speed_bumped_this_turn"] = 1
	// The +X/+0 anthem is a layer-7c effect that reads speed live; refresh
	// the characteristics cache so the new speed is observed.
	gs.InvalidateCharacteristicsCache()
}

func samutClearTurnGate(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	active, _ := ctx["active_seat"].(int)
	if active != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Flags == nil {
		return
	}
	delete(seat.Flags, "speed_bumped_this_turn")
}

// samutRegisterAnthemLayer wires "Other creatures you control get +X/+0,
// where X is your speed" as a §613 layer-7c POWER-ONLY continuous effect whose
// amount is read live from seat.Flags["speed"] on every layer pass. This
// replaces the old one-shot Modifications snapshot (which leaked after Samut
// left, didn't track creatures entering later, and re-stamped deltas by hand).
// SourcePerm = Samut so it auto-cleans on LTB; the speed counter itself is a
// player property and is intentionally left untouched here.
func samutRegisterAnthemLayer(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	lord := perm
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerPT,
		Sublayer:       "c",
		SourcePerm:     lord,
		SourceCardName: lord.Card.DisplayName(),
		ControllerSeat: lord.Controller,
		HandlerID:      "samut_speed_anthem:" + itoa(lord.Timestamp),
		Duration:       gameengine.DurationPermanent,
		Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
			return t != nil && t != lord && t.Card != nil &&
				t.Controller == lord.Controller && t.IsCreature()
		},
		ApplyFn: func(g *gameengine.GameState, _ *gameengine.Permanent, chars *gameengine.Characteristics) {
			seat := g.Seats[lord.Controller]
			if seat == nil {
				return
			}
			x := seat.Flags["speed"]
			if x > 0 {
				chars.Power += x
			}
		},
	})
	gs.InvalidateCharacteristicsCache()
}
