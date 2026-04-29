package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDavros wires Davros, Dalek Creator.
//
// "Whenever a player loses the game, each opponent gets three rad counters."
//
// This triggers on seat_eliminated events and gives 3 rad counters to
// each remaining opponent of the eliminated player (i.e., every other
// living player).
func registerDavros(r *Registry) {
	r.OnTrigger("Davros, Dalek Creator", "seat_eliminated", davrosTrigger)
}

func davrosTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if perm == nil || gs == nil {
		return
	}
	// Davros just needs to be on the battlefield (which it is, since
	// fireTrigger only walks battlefield permanents).
	controller := perm.Controller
	if controller < 0 || controller >= len(gs.Seats) {
		return
	}

	// Give 3 rad counters to each opponent of Davros's controller.
	// "each opponent" means every living player other than the controller.
	for i, s := range gs.Seats {
		if s == nil || s.Lost || i == controller {
			continue
		}
		if s.Flags == nil {
			s.Flags = map[string]int{}
		}
		s.Flags["rad_counters"] += 3
		gs.LogEvent(gameengine.Event{
			Kind:   "counter_mod",
			Seat:   controller,
			Target: i,
			Source: "Davros, Dalek Creator",
			Amount: 3,
			Details: map[string]interface{}{
				"counter_kind": "rad",
				"op":           "put",
				"on_player":    true,
				"reason":       "davros_trigger",
			},
		})
	}
}
