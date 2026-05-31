package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerHopeEstheim wires Hope Estheim.
//
// Oracle text:
//
//	Lifelink
//	At the beginning of your end step, each opponent mills X cards,
//	where X is the amount of life you gained this turn.
//
// Implementation:
//   - life_gained: accumulate per-turn life-gain on perm.Flags scoped
//     to the current turn.
//   - end_step_controller / end_step: at controller's end step, mill X
//     from each living opponent.
func registerHopeEstheim(r *Registry) {
	r.OnTrigger("Hope Estheim", "life_gained", hopeEstheimLifeGain)
	r.OnTrigger("Hope Estheim", "end_step", hopeEstheimEndStep)
}

func hopeEstheimLifeGain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	const turnKey = "hope_estheim_turn"
	const totalKey = "hope_estheim_total"
	if perm.Flags[turnKey] != gs.Turn {
		perm.Flags[turnKey] = gs.Turn
		perm.Flags[totalKey] = 0
	}
	perm.Flags[totalKey] += amount
}

func hopeEstheimEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "hope_estheim_mill"
	if gs == nil || perm == nil {
		return
	}
	if active, ok := ctx["active_seat"].(int); ok {
		if active != perm.Controller {
			return
		}
	} else if gs.Active != perm.Controller {
		return
	}
	if perm.Flags == nil {
		return
	}
	if perm.Flags["hope_estheim_turn"] != gs.Turn {
		return
	}
	x := perm.Flags["hope_estheim_total"]
	if x <= 0 {
		return
	}
	for i := range gs.Seats {
		if i == perm.Controller {
			continue
		}
		s := gs.Seats[i]
		if s == nil || s.Lost {
			continue
		}
		n := x
		if n > len(s.Library) {
			n = len(s.Library)
		}
		if n <= 0 {
			continue
		}
		milled := append([]*gameengine.Card(nil), s.Library[:n]...)
		for _, c := range milled {
			gameengine.MoveCard(gs, c, i, "library", "graveyard", "hope_estheim_mill")
		}
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"x":    x,
	})
}
