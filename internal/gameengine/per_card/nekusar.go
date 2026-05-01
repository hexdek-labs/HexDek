package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNekusar wires Nekusar, the Mindrazer.
//
// Oracle text:
//
//	At the beginning of each player's draw step, that player draws an
//	additional card.
//	Whenever an opponent draws a card, Nekusar, the Mindrazer deals 1
//	damage to that player.
//
// Implementation:
//   - OnTrigger "draw_step_controller": fires for the active player's
//     draw step (CR §504). The active player draws one extra card. Per
//     the wording "each player's draw step", this fires for every player
//     in turn order — the engine emits draw_step_controller exactly once
//     per turn (active_seat ctx), so cycling around the table covers all
//     four players naturally. If the extra-draw recipient is an opponent
//     of Nekusar's controller, we also ping them for 1 (folding the
//     "opponent draws" trigger inline since the in-engine "card_drawn"
//     dispatch path is hardcoded in cast_counts.go and does not call
//     into per_card for arbitrary names).
//   - OnTrigger "card_drawn" / "opponent_draws": parity registration
//     mirroring Consecrated Sphinx / Queza so the handler fires in test
//     fixtures that emit those events directly.
func registerNekusar(r *Registry) {
	r.OnTrigger("Nekusar, the Mindrazer", "draw_step_controller", nekusarDrawStep)
	r.OnTrigger("Nekusar, the Mindrazer", "card_drawn", nekusarOpponentDraw)
	r.OnTrigger("Nekusar, the Mindrazer", "opponent_draws", nekusarOpponentDraw)
}

func nekusarDrawStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nekusar_draw_step_extra"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[activeSeat]
	if s == nil || s.Lost {
		return
	}
	drawn := drawOne(gs, activeSeat, perm.Card.DisplayName())
	if drawn == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"active_seat": activeSeat,
			"drawn":       0,
		})
		return
	}
	// Inline opponent-draw ping for the extra draw, since the engine's
	// FireDrawTriggerObservers has a hardcoded card-name switch.
	pinged := nekusarPing(gs, perm, activeSeat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"active_seat": activeSeat,
		"drawn":       1,
		"pinged":      pinged,
	})
	_ = gs.CheckEnd()
}

func nekusarOpponentDraw(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "nekusar_opponent_draw_ping"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	drawerSeat, ok := ctx["drawer_seat"].(int)
	if !ok {
		return
	}
	if drawerSeat == perm.Controller {
		return
	}
	pinged := nekusarPing(gs, perm, drawerSeat)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"drawer_seat": drawerSeat,
		"pinged":      pinged,
	})
	_ = gs.CheckEnd()
}

// nekusarPing deals 1 damage to drawerSeat from Nekusar's controller,
// returning true if damage was applied. No-op when drawer is Nekusar's
// own controller or already lost.
func nekusarPing(gs *gameengine.GameState, perm *gameengine.Permanent, drawerSeat int) bool {
	if drawerSeat < 0 || drawerSeat >= len(gs.Seats) {
		return false
	}
	if drawerSeat == perm.Controller {
		return false
	}
	tgt := gs.Seats[drawerSeat]
	if tgt == nil || tgt.Lost {
		return false
	}
	tgt.Life -= 1
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   perm.Controller,
		Target: drawerSeat,
		Source: perm.Card.DisplayName(),
		Amount: 1,
		Details: map[string]interface{}{
			"reason":      "nekusar_opponent_draw",
			"drawer_seat": drawerSeat,
			"target_kind": "player",
		},
	})
	return true
}
