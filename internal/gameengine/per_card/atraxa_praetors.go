package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAtraxaPraetorsVoice wires Atraxa, Praetors' Voice.
//
// Oracle text:
//
//	Flying, vigilance, deathtouch, lifelink.
//	At the beginning of your end step, proliferate. (Choose any number
//	of permanents and/or players with counters, then give each another
//	counter of each kind already there.)
//
// Greedy AI policy mirrors the engine's stock proliferate (see
// resolve_helpers.go case "proliferate"):
//   - Add a counter of every existing kind to each permanent we control.
//   - For opponent permanents, skip beneficial "+1/+1" counters and only
//     proliferate harmful counters (-1/-1, stun, etc.).
//   - On the controller, proliferate energy/experience.
//   - On opponents, proliferate poison and rad.
func registerAtraxaPraetorsVoice(r *Registry) {
	r.OnTrigger("Atraxa, Praetors' Voice", "end_step", atraxaPraetorsEndStep)
}

func atraxaPraetorsEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "atraxa_praetors_voice_proliferate"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	if gs.Seats[seat] == nil || gs.Seats[seat].Lost {
		return
	}

	// Counter DB Phase 4 — delegate to the canonical Proliferate
	// primitive. BuildGreedyProliferateTargets applies the same
	// GreedyHat policy (skip opponent +1/+1, proliferate own
	// experience, proliferate opponent poison + rad) the inline
	// resolver case uses, so observable behavior is unchanged but the
	// path now flows through the §122 registry and InstanceID lineage.
	proliferated, _ := gameengine.Proliferate(gs, seat, gameengine.BuildGreedyProliferateTargets(gs, seat))
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"proliferated": proliferated,
	})
}
