package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFynn wires Fynn, the Fangbearer.
//
// "Whenever a creature you control with deathtouch deals combat damage
// to a player, that player gets two poison counters."
//
// This triggers on combat_damage_player events and checks if the
// source creature has deathtouch. If so, the damaged player gets 2
// additional poison counters.
func registerFynn(r *Registry) {
	r.OnTrigger("Fynn, the Fangbearer", "combat_damage_player", fynnTrigger)
}

func fynnTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if perm == nil || gs == nil {
		return
	}
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	// Get the source creature info from the trigger context.
	sourceSeat := -1
	sourceName := ""
	defenderSeat := -1
	if ctx != nil {
		if ss, ok := ctx["source_seat"].(int); ok {
			sourceSeat = ss
		}
		if sn, ok := ctx["source_card"].(string); ok {
			sourceName = sn
		}
		if ds, ok := ctx["defender_seat"].(int); ok {
			defenderSeat = ds
		}
	}

	// The source must be controlled by Fynn's controller.
	if sourceSeat != controller {
		return
	}
	if defenderSeat < 0 || defenderSeat >= len(gs.Seats) {
		return
	}

	// Find the source creature on the battlefield and check for deathtouch.
	hasDeathtouch := false
	for _, p := range gs.Seats[controller].Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !strings.EqualFold(p.Card.DisplayName(), sourceName) {
			continue
		}
		if p.HasKeyword("deathtouch") {
			hasDeathtouch = true
		}
		break
	}
	if !hasDeathtouch {
		return
	}

	// Add 2 poison counters to the damaged player.
	gs.Seats[defenderSeat].PoisonCounters += 2
	gs.LogEvent(gameengine.Event{
		Kind:   "poison",
		Seat:   controller,
		Target: defenderSeat,
		Source: "Fynn, the Fangbearer",
		Amount: 2,
		Details: map[string]interface{}{
			"reason":        "fynn_trigger",
			"source_card":   sourceName,
			"rule":          "702.165",
			"target_kind":   "player",
			"combat":        true,
		},
	})
}
