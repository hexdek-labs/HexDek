package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMinwuWhiteMage wires Minwu, White Mage.
//
// Oracle text:
//
//   Vigilance, lifelink
//   Whenever you gain life, put a +1/+1 counter on each Cleric you control.
//
// Auto-generated trigger handler.
func registerMinwuWhiteMage(r *Registry) {
	r.OnTrigger("Minwu, White Mage", "life_gained", minwuWhiteMageTrigger)
}

func minwuWhiteMageTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "minwu_white_mage_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller { return }
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	for _, p := range gs.Seats[perm.Controller].Battlefield {
		if p == nil || !p.IsCreature() || p == perm { continue }
		p.AddCounter("+1/+1", 1)
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
