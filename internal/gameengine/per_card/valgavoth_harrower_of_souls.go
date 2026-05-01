package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerValgavothHarrowerOfSouls wires Valgavoth, Harrower of Souls.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Flying
//	Ward—Pay 2 life.
//	Whenever an opponent loses life for the first time during each of
//	their turns, put a +1/+1 counter on Valgavoth and draw a card.
//
// Implementation:
//   - Flying / Ward—Pay 2 life: handled by the AST keyword pipeline.
//   - "life_lost": fires per life-loss event with ctx["seat"] == loser
//     and ctx["amount"] > 0. Gating:
//       * The losing seat must be an opponent of Valgavoth's controller.
//       * It must be the losing seat's own turn (gs.Active == lossSeat).
//       * It must be the FIRST life-loss for that seat this turn.
//     The "first time per turn" gate is per-Valgavoth via a flag keyed
//     by (turn, lossSeat) on perm.Flags. Multiple Valgavoths on the
//     battlefield each grow once per opponent per turn (each tracks its
//     own gate), matching the multiple-trigger rule for separately
//     printed copies.
//   - On hit: +1/+1 counter on Valgavoth and draw a card.
func registerValgavothHarrowerOfSouls(r *Registry) {
	r.OnTrigger("Valgavoth, Harrower of Souls", "life_lost", valgavothHarrowerLifeLost)
}

func valgavothHarrowerKey(turn, seat int) string {
	return "valgavoth_loss_t" + strconv.Itoa(turn+1) + "_s" + strconv.Itoa(seat)
}

func valgavothHarrowerLifeLost(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "valgavoth_harrower_of_souls_grow"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	lossSeat, ok := ctx["seat"].(int)
	if !ok || lossSeat < 0 || lossSeat >= len(gs.Seats) {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	// Must be an opponent of Valgavoth's controller.
	if lossSeat == perm.Controller {
		return
	}
	loser := gs.Seats[lossSeat]
	if loser == nil || loser.Lost {
		return
	}
	// "during each of their turns" — only on the losing seat's own turn.
	if gs.Active != lossSeat {
		return
	}

	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	key := valgavothHarrowerKey(gs.Turn, lossSeat)
	if perm.Flags[key] == 1 {
		return
	}
	perm.Flags[key] = 1

	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	drawn := drawOne(gs, perm.Controller, perm.Card.DisplayName())

	drawnName := ""
	if drawn != nil {
		drawnName = drawn.DisplayName()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"loss_seat":    lossSeat,
		"loss_amount":  amount,
		"drawn_card":   drawnName,
		"counters":     perm.Counters["+1/+1"],
	})
}
