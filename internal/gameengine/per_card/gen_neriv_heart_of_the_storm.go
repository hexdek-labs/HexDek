package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerNerivHeartOfTheStorm wires Neriv, Heart of the Storm.
//
// Oracle text:
//
//	Flying
//	If a creature you control that entered this turn would deal damage,
//	it deals twice that much damage instead.
//
// Implementation:
//   - Flying: AST keyword pipeline.
//   - Damage doubling: engine-deep DealDamage replacement on creatures
//     with the "entered_this_turn" marker. Set per-seat flag, emit
//     partial. Approximate the marker on every permanent_etb event by
//     stamping the entering perm with Flags["neriv_doubles_damage"]=1
//     when Neriv is on its controller's battlefield. The engine's
//     end-of-turn cleanup should clear that flag.
func registerNerivHeartOfTheStorm(r *Registry) {
	r.OnETB("Neriv, Heart of the Storm", nerivETBSetSeatFlag)
	r.OnTrigger("Neriv, Heart of the Storm", "permanent_etb", nerivStampEnteringCreature)
	r.OnTrigger("Neriv, Heart of the Storm", "end_step", nerivClearMarkers)
	// R51 batch I: LTB clears the seat-level damage-doubler activation
	// flag + sweeps all per-perm neriv_doubles_damage markers so a
	// removed Neriv doesn't leave the replacement contract active.
	r.OnTrigger("Neriv, Heart of the Storm", "permanent_ltb", nerivLTBClearFlags)
}

func nerivLTBClearFlags(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags != nil {
		delete(seat.Flags, "neriv_double_etb_damage_active")
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		delete(p.Flags, "neriv_doubles_damage")
	}
}

func nerivETBSetSeatFlag(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "neriv_heart_of_the_storm_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags["neriv_double_etb_damage_active"] = 1
	// Stamp Neriv himself + every creature already on the battlefield
	// that entered this turn — best-effort: we don't track entered-this-turn
	// for existing permanents, so we mark only Neriv on ETB.
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["neriv_doubles_damage"] = 1
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"damage doubling needs DealDamage replacement hook; per-perm marker stamped for downstream consumers")
}

func nerivStampEnteringCreature(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	newcomer, _ := ctx["perm"].(*gameengine.Permanent)
	if newcomer == nil || newcomer.Card == nil || !newcomer.IsCreature() {
		return
	}
	if newcomer.Controller != perm.Controller {
		return
	}
	if newcomer.Flags == nil {
		newcomer.Flags = map[string]int{}
	}
	newcomer.Flags["neriv_doubles_damage"] = 1
}

func nerivClearMarkers(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	for _, p := range seat.Battlefield {
		if p == nil || p.Flags == nil {
			continue
		}
		delete(p.Flags, "neriv_doubles_damage")
	}
}
