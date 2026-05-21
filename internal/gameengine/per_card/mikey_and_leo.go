package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMikeyAndLeo wires Mikey & Leo, Chaos & Order.
//
// Oracle text:
//
//	Whenever you put a counter on a creature you control, draw a card.
//	This ability triggers only once each turn.
//
// Implementation (R53 batch N port):
//   - counter_placed trigger IS dispatched by resolve.go on every
//     successful counter-put. Gate on (a) source_seat ==
//     controller, (b) target_perm is a creature the controller
//     controls, and (c) the per-turn flag (perm.Flags["mikey_drawn_turn"])
//     hasn't been bumped this turn.
//   - On first qualifying trigger per turn, draw 1 card and mark
//     the perm flag with gs.Turn+1 as sentinel.
func registerMikeyAndLeo(r *Registry) {
	r.OnETB("Mikey & Leo, Chaos & Order", mikeyAndLeoETB)
	r.OnTrigger("Mikey & Leo, Chaos & Order", "counter_placed", mikeyAndLeoCounterPlaced)
}

func mikeyAndLeoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "mikey_and_leo_etb"
	if gs == nil || perm == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func mikeyAndLeoCounterPlaced(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "mikey_and_leo_counter_placed_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	srcSeat, _ := ctx["source_seat"].(int)
	if srcSeat != perm.Controller {
		return
	}
	target, _ := ctx["target_perm"].(*gameengine.Permanent)
	if target == nil || target.Card == nil {
		return
	}
	if target.Controller != perm.Controller || !target.IsCreature() {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	sentinel := gs.Turn + 1
	if perm.Flags["mikey_drawn_turn"] == sentinel {
		return
	}
	perm.Flags["mikey_drawn_turn"] = sentinel
	drawOne(gs, perm.Controller, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"target": target.Card.DisplayName(),
	})
}
