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
	r.OnTrigger("Samut, the Driving Force", "permanent_etb", samutRefreshAnthem)
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
	samutApplyAnthem(gs, perm, seat)
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
	samutApplyAnthem(gs, perm, seat)
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

func samutRefreshAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	samutApplyAnthem(gs, perm, seat)
}

func samutApplyAnthem(gs *gameengine.GameState, perm *gameengine.Permanent, seat *gameengine.Seat) {
	x := seat.Flags["speed"]
	if x <= 0 {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil || !p.IsCreature() {
			continue
		}
		// Idempotent stamp via a marker flag — we use the speed value as
		// the marker so when speed changes we re-stamp.
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		stamped := p.Flags["samut_anthem_speed"]
		if stamped == x {
			continue
		}
		// Add (x - stamped) power-only delta. If the stamp went down
		// (rare), apply a negative delta.
		delta := x - stamped
		p.Modifications = append(p.Modifications, gameengine.Modification{
			Power:     delta,
			Toughness: 0,
			Duration:  "while_source_on_battlefield",
			Timestamp: gs.NextTimestamp(),
		})
		p.Flags["samut_anthem_speed"] = x
	}
	gs.InvalidateCharacteristicsCache()
}
