package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Zedruu the Greathearted — {1}{U}{R}{W} 2/4 Legendary Creature — Minotaur
// Monk.
//
//   At the beginning of your upkeep, you gain X life and draw X cards,
//   where X is the number of permanents you own that your opponents
//   control.
//   {U}{R}{W}: Target opponent gains control of target permanent you
//   control.
//
// Implementation:
//   - OnTrigger upkeep_controller: scan every opponent's battlefield for
//     permanents where p.Owner == Zedruu's controller; X = count;
//     GainLife(X) + drawOne X times.
//   - Activated ability: control-grab gift needs a conditional-duration
//     continuous-effect ("for as long as Zedruu remains on the battlefield"
//     is NOT in this oracle — the Zedruu gift is permanent unless reversed
//     by another effect); however the actual control-change machinery is
//     engine territory (gain-control of your own permanent → opp's
//     battlefield). emitPartial flags it for future engine work.

func init() {
	registerZedruuTheGreathearted(Global())
	AddResetHook(registerZedruuTheGreathearted)
}

func registerZedruuTheGreathearted(r *Registry) {
	r.OnTrigger("Zedruu the Greathearted", "upkeep_controller", zedruuUpkeep)
	r.OnActivated("Zedruu the Greathearted", zedruuActivate)
}

func zedruuUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zedruu_upkeep_payoff"
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
	x := 0
	for i, opp := range gs.Seats {
		if opp == nil || i == seat {
			continue
		}
		for _, p := range opp.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.Owner == seat {
				x++
			}
		}
	}
	if x <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":            seat,
			"x":               0,
			"life_gained":     0,
			"cards_drawn":     0,
		})
		return
	}
	gameengine.GainLife(gs, seat, x, perm.Card.DisplayName())
	drawn := 0
	for i := 0; i < x; i++ {
		if drawOne(gs, seat, perm.Card.DisplayName()) == nil {
			break
		}
		drawn++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         seat,
		"x":            x,
		"life_gained":  x,
		"cards_drawn":  drawn,
	})
}

func zedruuActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "zedruu_give_permanent"
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"gift_control_change_to_target_opponent_needs_engine_control_change_plumbing")
}
