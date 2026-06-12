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
// Implementation (R54 — damage replacement primitive):
//   - Flying: AST keyword pipeline.
//   - ETB: register a DamageReplacement closure that filters on
//     (ctx.Source != nil AND source.Controller == Neriv's controller
//     AND source is a creature AND source.Flags["neriv_doubles_damage"]
//     == 1). Per-permanent "entered this turn" marker is stamped on
//     Neriv himself at ETB and on every creature entering after Neriv
//     via the permanent_etb trigger; end_step clears the markers so
//     the rider's "this turn" duration is honored.
//   - LTB unregisters the closure and sweeps remaining markers.
func registerNerivHeartOfTheStorm(r *Registry) {
	r.OnETB("Neriv, Heart of the Storm", nerivETBSetSeatFlag)
	r.OwnsETBTrigger("Neriv, Heart of the Storm")
	r.OnTrigger("Neriv, Heart of the Storm", "permanent_etb", nerivStampEnteringCreature)
	r.OnTrigger("Neriv, Heart of the Storm", "end_step", nerivClearMarkers)
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
	gs.UnregisterDamageReplacementsForPermanent(perm)
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
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["neriv_doubles_damage"] = 1

	controller := perm.Controller
	gs.RegisterDamageReplacement(&gameengine.DamageReplacement{
		SourcePerm: perm,
		HandlerID:  "neriv_double_entered_this_turn",
		Fn: func(gs *gameengine.GameState, ctx *gameengine.DamageContext) {
			if ctx == nil || ctx.Source == nil {
				return
			}
			if ctx.Source.Controller != controller {
				return
			}
			if !ctx.Source.IsCreature() {
				return
			}
			if ctx.Source.Flags == nil || ctx.Source.Flags["neriv_doubles_damage"] != 1 {
				return
			}
			if ctx.Amount <= 0 {
				return
			}
			ctx.Amount *= 2
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
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
