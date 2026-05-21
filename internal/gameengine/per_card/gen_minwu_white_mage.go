package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMinwuWhiteMage wires Minwu, White Mage.
//
// Oracle text (Scryfall, verified):
//
//	Vigilance, lifelink
//	Whenever you gain life, put a +1/+1 counter on each Cleric you
//	control.
//
// Implementation (R49 stub port):
//   - Vigilance + lifelink: AST keyword pipeline.
//   - Life-gain trigger: the auto-gen handler put a +1/+1 counter on
//     EVERY non-self creature, ignoring the Cleric filter (printed
//     oracle says "each Cleric"). Restrict to permanents whose Card
//     type-line / Types contains "cleric". Also count Minwu himself
//     when he's a Cleric (he isn't on the printed type-line, but the
//     code keeps the symmetric "p != perm" filter so the source
//     doesn't double-buff itself even if engine-side type-grants make
//     him a Cleric later).
func registerMinwuWhiteMage(r *Registry) {
	r.OnTrigger("Minwu, White Mage", "life_gained", minwuWhiteMageTrigger)
}

func minwuWhiteMageTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "minwu_white_mage_lifegain_cleric_buff"
	if gs == nil || perm == nil || perm.Card == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	bumped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if !cardHasType(p.Card, "cleric") {
			continue
		}
		p.AddCounter("+1/+1", 1)
		bumped++
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":    perm.Controller,
		"clerics": bumped,
	})
}
