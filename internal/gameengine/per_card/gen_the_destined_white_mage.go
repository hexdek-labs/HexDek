package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTheDestinedWhiteMage wires The Destined White Mage.
//
// Oracle text:
//
//   Lifelink
//   {W}, {T}: Another target creature you control gains lifelink until end of turn.
//   Whenever you gain life, put a +1/+1 counter on target creature you control. If you have a full party, put three +1/+1 counters on that creature instead.
//
// Auto-generated trigger handler.
func registerTheDestinedWhiteMage(r *Registry) {
	r.OnTrigger("The Destined White Mage", "life_gained", theDestinedWhiteMageTrigger)
}

func theDestinedWhiteMageTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "the_destined_white_mage_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller { return }
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	perm.AddCounter("+1/+1", 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
