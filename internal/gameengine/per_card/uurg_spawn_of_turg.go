package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerUurgSpawnOfTurg wires Uurg, Spawn of Turg.
//
// Oracle text:
//
//	{B}{B}{G}
//	Legendary Creature — Frog Beast
//	Uurg's power is equal to the number of land cards in your graveyard.
//	At the beginning of your upkeep, surveil 1.
//	{B}{G}, Sacrifice a land: You gain 2 life.
//
// Implementation:
//   - R55: power CDA via RegisterDynamicSetPower (Layer 7b sublayer).
//     The compute fn re-evaluates the graveyard land count on every
//     layer pass, so power tracks live state without explicit refresh.
//   - Upkeep: surveil 1 via gameengine.Surveil.
//   - Activated {B}{G} sac-land → 2 life remains partial (engine-deep
//     activated-cost dispatch).
func registerUurgSpawnOfTurg(r *Registry) {
	r.OnETB("Uurg, Spawn of Turg", uurgETB)
	r.OnTrigger("Uurg, Spawn of Turg", "upkeep_controller", uurgUpkeep)
}

func uurgETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	gameengine.RegisterDynamicSetPower(gs, perm, uurgCountLandsInGraveyard,
		gameengine.DurationUntilSourceLeaves, "Uurg, Spawn of Turg", "cda_power")
	gs.InvalidateCharacteristicsCache()
	emit(gs, "uurg_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, "uurg_etb", perm.Card.DisplayName(),
		"activated_BG_sac_land_gain_2_partial")
}

func uurgUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "uurg_upkeep_surveil"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	gameengine.Surveil(gs, perm.Controller, 1)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
}

func uurgCountLandsInGraveyard(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0
	}
	n := 0
	for _, c := range seat.Graveyard {
		if c == nil {
			continue
		}
		if cardHasType(c, "land") {
			n++
		}
	}
	return n
}
