package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZndrspltEyeOfWisdom wires Zndrsplt, Eye of Wisdom.
//
// Oracle text:
//
//	Partner with Okaun, Eye of Chaos
//	At the beginning of combat on your turn, flip a coin until you lose
//	a flip.
//	Whenever a player wins a coin flip, draw a card.
//
// Implementation:
//   - "combat_begin" gated on active_seat == perm.Controller: flip coins
//     until we lose. Each won flip fires the draw trigger inline. We
//     bound the loop at 32 iterations to prevent unbounded loops on
//     coin-flip-rigging effects (none implemented).
//   - The "whenever a player wins a coin flip" trigger is implemented by
//     calling drawOne directly within the loop. A more orthogonal
//     implementation would use a coin_flip event; engine doesn't expose
//     a per_card hook for it cleanly, so we inline.
func registerZndrspltEyeOfWisdom(r *Registry) {
	r.OnTrigger("Zndrsplt, Eye of Wisdom", "combat_begin", zndrspltCombatBegin)
}

func zndrspltCombatBegin(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "zndrsplt_combat_flip"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	wins := 0
	for i := 0; i < 32; i++ {
		flip := rngIntn(gs, 2)
		if flip == 0 {
			break
		}
		wins++
		drawOne(gs, perm.Controller, perm.Card.DisplayName())
		gs.LogEvent(gameengine.Event{
			Kind:   "coin_flip",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"slug":   slug,
				"won":    true,
				"flip_n": i + 1,
			},
		})
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
		"wins": wins,
	})
}
