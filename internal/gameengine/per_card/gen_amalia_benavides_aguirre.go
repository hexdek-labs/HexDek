package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAmaliaBenavidesAguirre wires Amalia Benavides Aguirre.
//
// Oracle text:
//
//   Ward—Pay 3 life.
//   Whenever you gain life, Amalia Benavides Aguirre explores. Then destroy all other creatures if its power is exactly 20. (To have this creature explore, reveal the top card of your library. Put that card into your hand if it's a land. Otherwise, put a +1/+1 counter on this creature, then put the card back or put it into your graveyard.)
//
// Auto-generated trigger handler.
func registerAmaliaBenavidesAguirre(r *Registry) {
	r.OnTrigger("Amalia Benavides Aguirre", "life_gained", amaliaBenavidesAguirreTrigger)
}

func amaliaBenavidesAguirreTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "amalia_benavides_aguirre_trigger"
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
