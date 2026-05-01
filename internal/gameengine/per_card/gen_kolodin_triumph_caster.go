package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKolodinTriumphCaster wires Kolodin, Triumph Caster.
//
// Oracle text:
//
//   Mounts and Vehicles you control have haste.
//   Whenever a Mount you control enters, it becomes saddled until end of turn.
//   Whenever a Vehicle you control enters, it becomes an artifact creature until end of turn.
//
// Auto-generated trigger handler.
func registerKolodinTriumphCaster(r *Registry) {
	r.OnTrigger("Kolodin, Triumph Caster", "permanent_etb", kolodinTriumphCasterTrigger1)
	r.OnTrigger("Kolodin, Triumph Caster", "permanent_etb", kolodinTriumphCasterTrigger2)
}

func kolodinTriumphCasterTrigger1(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kolodin_triumph_caster_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller { return }
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: trigger effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func kolodinTriumphCasterTrigger2(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kolodin_triumph_caster_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller { return }
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: trigger effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
