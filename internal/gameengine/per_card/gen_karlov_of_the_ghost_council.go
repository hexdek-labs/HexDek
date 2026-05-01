package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKarlovOfTheGhostCouncil wires Karlov of the Ghost Council.
//
// Oracle text:
//
//   Whenever you gain life, put two +1/+1 counters on Karlov.
//   {W}{B}, Remove six +1/+1 counters from Karlov: Exile target creature.
//
// Auto-generated trigger handler.
func registerKarlovOfTheGhostCouncil(r *Registry) {
	r.OnTrigger("Karlov of the Ghost Council", "life_gained", karlovOfTheGhostCouncilTrigger)
}

func karlovOfTheGhostCouncilTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "karlov_of_the_ghost_council_trigger"
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
