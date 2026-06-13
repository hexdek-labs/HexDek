package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// gaze_of_adamaro_r63.go — per_card handler for Gaze of Adamaro.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Gaze of Adamaro deals damage to target player equal to the number of
//	cards in that player's hand.
//
// {2}{R} Instant — Arcane. Punishes a full grip. Parses to an inert
// fallback node (no structured damage), and the runtime text fallback has
// no "damage equal to target's hand size" shape — fully DOA.
//
// Implementation: OnResolve (authoritative). Pick the opponent holding
// the MOST cards (maximally effective for the AI) and deal that many.
func init() {
	registerGazeOfAdamaroR63(Global())
	AddResetHook(registerGazeOfAdamaroR63)
}

func registerGazeOfAdamaroR63(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Gaze of Adamaro", gazeOfAdamaroResolve)
}

func gazeOfAdamaroResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "gaze_of_adamaro"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	// Target the opponent with the largest hand.
	target, bestHand := -1, -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if len(os.Hand) > bestHand {
			bestHand = len(os.Hand)
			target = opp
		}
	}

	dmg := 0
	if target >= 0 {
		dmg = len(gs.Seats[target].Hand)
		if dmg > 0 {
			gameengine.DealDamage(gs, target, dmg, "Gaze of Adamaro")
		}
	}
	emit(gs, slug, "Gaze of Adamaro", map[string]interface{}{
		"seat": seat, "target": target, "damage": dmg,
	})
}
