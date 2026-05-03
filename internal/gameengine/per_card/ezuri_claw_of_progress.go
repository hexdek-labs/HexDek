package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEzuriClawOfProgress wires Ezuri, Claw of Progress.
//
// Oracle text (Scryfall, verified 2026-05-02):
//
//	Whenever a creature with power 2 or less you control enters, you get
//	an experience counter.
//	At the beginning of combat on your turn, put X +1/+1 counters on
//	another target creature you control, where X is the number of
//	experience counters you have.
//
// Experience counters live in seat.Flags["experience_counters"], matching
// the engine's existing experience-counter wiring (meren.go,
// mizzix_of_the_izmagnus.go, resolve_helpers.go, scaling.go, atraxa_praetors.go)
// so proliferate and ScalingAmount references see the same value.
//
// Implementation:
//   - OnTrigger("permanent_etb"): fires whenever any permanent enters the
//     battlefield. Gate on: entering permanent is a creature, its power <= 2,
//     and its controller is Ezuri's controller. Increment
//     seat.Flags["experience_counters"] by 1.
//   - OnTrigger("combat_begin"): at the beginning of combat on Ezuri's
//     controller's active turn, read X = experience_counters from the seat,
//     then place X +1/+1 counters on the "best" other creature the controller
//     controls (highest combined power + toughness, tiebreak by lowest
//     Timestamp). If X == 0, no counters are placed but the trigger is logged.
//   - De-dupe on combat_begin via turn-keyed perm.Flags sentinel to prevent
//     double-fire in extra-combat phases.
//
// emitPartial: none — both clauses are fully modelled.
func registerEzuriClawOfProgress(r *Registry) {
	r.OnTrigger("Ezuri, Claw of Progress", "permanent_etb", ezuriPermETB)
	r.OnTrigger("Ezuri, Claw of Progress", "combat_begin", ezuriCombatBegin)
}

// ezuriPermETB fires whenever any permanent enters the battlefield.
// It awards an experience counter when the entering creature is controlled
// by Ezuri's controller and has power 2 or less.
func ezuriPermETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	entering, _ := ctx["perm"].(*gameengine.Permanent)
	if entering == nil || entering.Card == nil {
		return
	}
	// Only trigger for our own creatures entering.
	enteringSeat, _ := ctx["controller_seat"].(int)
	if enteringSeat != perm.Controller {
		return
	}
	if !entering.IsCreature() {
		return
	}
	// "with power 2 or less" — gate on base + applied modifications.
	if entering.Power() > 2 {
		return
	}

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["experience_counters"]++

	gs.LogEvent(gameengine.Event{
		Kind:   "experience_counter",
		Seat:   perm.Controller,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason":    "ezuri_small_creature_etb",
			"trigger":   entering.Card.DisplayName(),
			"power":     entering.Power(),
			"total":     seat.Flags["experience_counters"],
		},
	})
	emit(gs, "ezuri_claw_of_progress_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"entering":       entering.Card.DisplayName(),
		"entering_power": entering.Power(),
		"xp_total":       seat.Flags["experience_counters"],
	})
}

// ezuriCombatBegin fires at the beginning of combat on Ezuri's controller's
// turn, placing X +1/+1 counters on the best other creature they control,
// where X = experience counters.
func ezuriCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "ezuri_claw_of_progress_combat"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: only fires on Ezuri's controller's active turn.
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}

	// De-dupe: extra-combat phases (Aggravated Assault, etc.) must not
	// fire a second time on the same turn. Sentinel stored as turn+1 so
	// the zero-value default is unambiguous even on turn 0.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	dedupeKey := "ezuri_combat_turn_" + strconv.Itoa(gs.Turn)
	if perm.Flags[dedupeKey] > 0 {
		return
	}
	perm.Flags[dedupeKey] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	x := 0
	if seat.Flags != nil {
		x = seat.Flags["experience_counters"]
	}

	// Find the best "another target creature you control" — highest combined
	// (power + toughness), tiebreak by lowest Timestamp (most-established).
	target := ezuriPickBestTarget(gs, perm)
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_other_creature", map[string]interface{}{
			"seat": perm.Controller,
			"x":    x,
		})
		return
	}

	if x > 0 {
		target.AddCounter("+1/+1", x)
		gs.InvalidateCharacteristicsCache()
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"target":           target.Card.DisplayName(),
		"x":                x,
		"counters_added":   x,
		"plus_one_total":   target.Counters["+1/+1"],
		"experience_total": x,
	})
}

// ezuriPickBestTarget returns the highest-value "another creature you control"
// for Ezuri's combat trigger. Highest (power + toughness) wins; tiebreak by
// lowest Timestamp (most-established permanent on the battlefield).
// Returns nil if no eligible target exists.
func ezuriPickBestTarget(gs *gameengine.GameState, ezuri *gameengine.Permanent) *gameengine.Permanent {
	if gs == nil || ezuri == nil {
		return nil
	}
	seat := gs.Seats[ezuri.Controller]
	if seat == nil {
		return nil
	}

	var best *gameengine.Permanent
	bestScore := -1 << 30
	bestTS := 1<<62 - 1

	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p == ezuri {
			continue
		}
		if !p.IsCreature() {
			continue
		}
		score := p.Power() + p.Toughness()
		if score > bestScore || (score == bestScore && p.Timestamp < bestTS) {
			bestScore = score
			bestTS = p.Timestamp
			best = p
		}
	}
	return best
}
