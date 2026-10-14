package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerChainerNightmareAdept wires Chainer, Nightmare Adept.
//
// Oracle text:
//
//	Discard a card: You may cast a creature spell from your graveyard
//	this turn. Activate only once each turn.
//	Whenever a nontoken creature you control enters, if you didn't cast
//	it from your hand, it gains haste until your next turn.
//
// The reanimator activated cost requires a stack-resolution path that
// the engine doesn't expose for free-cast-from-graveyard. Per-permanent
// haste grant on ETB is handled here.
func registerChainerNightmareAdept(r *Registry) {
	r.OnTrigger("Chainer, Nightmare Adept", "permanent_etb", chainerNightmareGrantHaste)
	r.OnActivated("Chainer, Nightmare Adept", chainerNightmareActivate)
}

func chainerNightmareGrantHaste(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "chainer_nightmare_haste"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || !entered.IsCreature() || entered.Card == nil {
		return
	}
	if cardHasType(entered.Card, "token") {
		return
	}
	source, _ := ctx["from_zone"].(string)
	if source == "hand" {
		return
	}
	if entered.Flags == nil {
		entered.Flags = map[string]int{}
	}
	gameengine.SetKeywordFlags(gs, entered, "haste")
	entered.SummoningSick = false
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"creature": entered.Card.DisplayName(),
	})
}

func chainerNightmareActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "chainer_nightmare_discard_to_cast"
	if gs == nil || src == nil {
		return
	}
	emitPartial(gs, slug, src.Card.DisplayName(),
		"discard_to_cast_creature_from_graveyard_unimplemented")
}
