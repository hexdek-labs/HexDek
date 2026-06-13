package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerWheelOfTorture wires Wheel of Torture.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	At the beginning of each opponent's upkeep, Wheel of Torture deals X damage
//	to that player, where X is 3 minus the number of cards in their hand.
//
// Why a per-card handler: the trigger parses to an inert `trigger_tail_fragment`
// (the whole "deals X damage ... 3 minus cards in hand" clause dumped as raw
// text) — generic dispatch deals no damage. A wheel/hand-denial punisher.
//
// Implementation (CR §603 triggered ability):
//   - Listen on "upkeep"; ctx "active_seat" = whose upkeep it is. Fire only on
//     an OPPONENT's upkeep (active_seat != controller).
//   - X = 3 - that opponent's hand size; deal X damage only when X > 0.
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerWheelOfTorture(Global())
	AddResetHook(registerWheelOfTorture)
}

func registerWheelOfTorture(r *Registry) {
	r.OnTrigger("Wheel of Torture", "upkeep", wheelOfTortureUpkeep)
}

func wheelOfTortureUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "wheel_of_torture_upkeep_damage"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	// Only on an opponent's upkeep.
	if activeSeat == perm.Controller {
		return
	}
	s := gs.Seats[activeSeat]
	if s == nil || s.Lost {
		return
	}

	x := 3 - len(s.Hand)
	if x <= 0 {
		return
	}
	gameengine.DealDamage(gs, activeSeat, x, perm.Card.DisplayName())

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"opponent":  activeSeat,
		"hand_size": len(s.Hand),
		"damage":    x,
	})
}
