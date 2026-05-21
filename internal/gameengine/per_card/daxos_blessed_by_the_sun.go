package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerDaxosBlessedByTheSun wires Daxos, Blessed by the Sun.
//
// Oracle text:
//
//	Daxos's toughness is equal to your devotion to white.
//	Whenever another creature you control enters or dies, you gain 1
//	life.
//
// The toughness CDA is left to the AST/state-resolver pipeline. We wire
// the gain-life trigger here.
func registerDaxosBlessedByTheSun(r *Registry) {
	r.OnETB("Daxos, Blessed by the Sun", daxosETBRegisterCDA)
	r.OnTrigger("Daxos, Blessed by the Sun", "permanent_etb", daxosCreatureETB)
	r.OnTrigger("Daxos, Blessed by the Sun", "creature_dies", daxosCreatureDies)
}

func daxosETBRegisterCDA(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "daxos_cda_layer7b"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// R55: toughness = devotion to white. Layer 7b dynamic toughness-only.
	gameengine.RegisterDynamicSetToughness(gs, perm, daxosDevotionWhite,
		gameengine.DurationUntilSourceLeaves, "Daxos, Blessed by the Sun", "cda_tough")
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func daxosDevotionWhite(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	return gameengine.DevotionToWhite(gs, perm.Controller)
}

func daxosCreatureETB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || !entered.IsCreature() {
		return
	}
	daxosGainLife(gs, perm)
}

func daxosCreatureDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if ctx == nil {
		return
	}
	deadController, _ := ctx["controller_seat"].(int)
	if deadController != perm.Controller {
		return
	}
	dead, _ := ctx["perm"].(*gameengine.Permanent)
	if dead == nil || dead == perm {
		return
	}
	daxosGainLife(gs, perm)
}

func daxosGainLife(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "daxos_blessed_lifegain"
	if perm.Controller < 0 || perm.Controller >= len(gs.Seats) {
		return
	}
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}
