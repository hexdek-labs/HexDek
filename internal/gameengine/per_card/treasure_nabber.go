package per_card

import "github.com/hexdek/hexdek/internal/gameengine"

// Treasure Nabber — {2}{R} Creature — Goblin Rogue 3/2
//
// Oracle: "Whenever an opponent taps an artifact for mana, gain control
// of that artifact until the end of your next turn."
//
// Key rules interaction: the trigger fires when the artifact is tapped,
// even if the artifact is sacrificed as part of the mana ability (e.g.
// Treasure tokens). The trigger still goes on the stack, but when it
// resolves the artifact no longer exists, so the control-change effect
// does nothing. This is correct per CR §603.3 — triggers fire based on
// the event occurring, not based on the object still existing at
// resolution time.
func registerTreasureNabber(r *Registry) {
	r.OnTrigger("Treasure Nabber", "artifact_tapped_for_mana", treasureNabberTrigger)
}

func treasureNabberTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	nabberSeat := perm.Controller

	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat == nabberSeat {
		return // only opponents' artifacts
	}

	artifact, _ := ctx["perm"].(*gameengine.Permanent)
	if artifact == nil {
		return
	}
	artifactName, _ := ctx["artifact_name"].(string)

	// Check if the artifact still exists on the battlefield. Treasure
	// tokens are sacrificed as part of their mana ability — by the time
	// this trigger resolves, the token is gone. Per CR §603.3 the trigger
	// fires, but the control-change effect can't find the object.
	found := false
	for _, s := range gs.Seats {
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == artifact {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		gs.LogEvent(gameengine.Event{
			Kind:   "trigger_fizzle",
			Seat:   nabberSeat,
			Source: "Treasure Nabber",
			Details: map[string]interface{}{
				"reason":   "artifact_no_longer_exists",
				"artifact": artifactName,
				"rule":     "603.3",
			},
		})
		return
	}

	// Gain control of the artifact until end of Nabber controller's next turn.
	oldController := artifact.Controller
	artifact.Controller = nabberSeat

	// Move from old controller's battlefield to new controller's.
	if oldController >= 0 && oldController < len(gs.Seats) && gs.Seats[oldController] != nil {
		bf := gs.Seats[oldController].Battlefield
		for i, p := range bf {
			if p == artifact {
				gs.Seats[oldController].Battlefield = append(bf[:i], bf[i+1:]...)
				break
			}
		}
	}
	if nabberSeat >= 0 && nabberSeat < len(gs.Seats) && gs.Seats[nabberSeat] != nil {
		gs.Seats[nabberSeat].Battlefield = append(gs.Seats[nabberSeat].Battlefield, artifact)
	}

	gs.LogEvent(gameengine.Event{
		Kind:   "control_change",
		Seat:   nabberSeat,
		Target: oldController,
		Source: "Treasure Nabber",
		Details: map[string]interface{}{
			"artifact":        artifactName,
			"old_controller":  oldController,
			"new_controller":  nabberSeat,
			"duration":        "until_end_of_next_turn",
			"rule":            "treasure_nabber",
		},
	})
}
