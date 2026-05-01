package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerFiresongAndSunspeaker wires Firesong and Sunspeaker.
//
// Oracle text:
//
//   Red instant and sorcery spells you control have lifelink.
//   Whenever a white instant or sorcery spell causes you to gain life, Firesong and Sunspeaker deals 3 damage to target creature or player.
//
// Auto-generated trigger handler.
func registerFiresongAndSunspeaker(r *Registry) {
	r.OnTrigger("Firesong and Sunspeaker", "life_gained", firesongAndSunspeakerTrigger)
}

func firesongAndSunspeakerTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "firesong_and_sunspeaker_trigger"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, _ := ctx["seat"].(int)
	if gainSeat != perm.Controller { return }
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
