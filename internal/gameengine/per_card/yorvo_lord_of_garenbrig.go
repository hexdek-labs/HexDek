package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerYorvoLordOfGarenbrig wires Yorvo, Lord of Garenbrig.
//
// Oracle text:
//
//	Yorvo enters with four +1/+1 counters on it.
//	Whenever another green creature you control enters, put a +1/+1
//	counter on Yorvo. Then if that creature's power is greater than
//	Yorvo's power, put another +1/+1 counter on Yorvo.
//
// Implementation:
//   - enters-with-counters: generic AST static path (per_card
//     duplicate deleted r63 — Yorvo was entering with 8).
//   - "permanent_etb" trigger: filter to other green creature controlled
//     by Yorvo's controller. Place 1 counter; if entering creature's
//     power exceeds Yorvo's, add a second counter.
func registerYorvoLordOfGarenbrig(r *Registry) {
	r.OnTrigger("Yorvo, Lord of Garenbrig", "permanent_etb", yorvoOtherGreenETB)
}

func yorvoOtherGreenETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "yorvo_lord_other_green_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if !entered.IsCreature() {
		return
	}
	isGreen := false
	for _, c := range entered.Card.Colors {
		if c == "G" || c == "g" {
			isGreen = true
			break
		}
	}
	if !isGreen {
		return
	}
	perm.AddCounter("+1/+1", 1)
	added := 1
	if entered.Power() > perm.Power() {
		perm.AddCounter("+1/+1", 1)
		added = 2
	}
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"entered":  entered.Card.DisplayName(),
		"counters": added,
	})
}
