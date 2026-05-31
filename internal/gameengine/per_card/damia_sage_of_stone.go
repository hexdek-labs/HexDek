package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Damia, Sage of Stone — {3}{B}{G}{U} 4/4 Legendary Creature — Gorgon Wizard.
//
//   Deathtouch
//   Skip your draw step.
//   At the beginning of your upkeep, if you have fewer than seven cards
//   in hand, draw cards equal to the difference.
//
// Implementation:
//   - OnETB: stamp gs.Flags["damia_skip_draw_seat_N"] for the phase-loop
//     skip-draw consumer (mirrors necropotence.go's latent-flag pattern;
//     phases.go consumes this if/when wired — see necropotence comment).
//   - OnTrigger upkeep_controller (active_seat == controller): refill hand
//     to seven via drawOne loop. Stops on empty library (AttemptedEmptyDraw
//     flag set by drawOne).
//
// Deathtouch is keyword-pipeline; not touched here.

func init() {
	registerDamiaSageOfStone(Global())
	AddResetHook(registerDamiaSageOfStone)
}

func registerDamiaSageOfStone(r *Registry) {
	r.OnETB("Damia, Sage of Stone", damiaETB)
	r.OnTrigger("Damia, Sage of Stone", "upkeep_controller", damiaUpkeep)
}

func damiaETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "damia_skip_draw_flag"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["damia_skip_draw_seat_"+intToStr(perm.Controller)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"skip_draw": 1,
	})
}

func damiaUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "damia_upkeep_refill_to_seven"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	handSize := len(s.Hand)
	if handSize >= 7 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":      seat,
			"hand_size": handSize,
			"drew":      0,
		})
		return
	}
	target := 7 - handSize
	drawn := 0
	for i := 0; i < target; i++ {
		if drawOne(gs, seat, perm.Card.DisplayName()) == nil {
			break
		}
		drawn++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           seat,
		"hand_size_pre":  handSize,
		"hand_size_post": len(s.Hand),
		"drew":           drawn,
	})
}
