package per_card

import (
	"strconv"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKolodinTriumphCaster wires Kolodin, Triumph Caster.
//
// Oracle text:
//
//	Mounts and Vehicles you control have haste.
//	Whenever a Mount you control enters, it becomes saddled until end of turn.
//	Whenever a Vehicle you control enters, it becomes an artifact creature
//	until end of turn.
//
// Implementation:
//   - Single permanent_etb trigger that classifies the entering permanent
//     and applies the right one-shot. Unified into one fn so we don't
//     fire two parser_gap breadcrumbs per ETB.
//   - Mount → set "saddled" flag through end of turn. Saddle-checking
//     code (e.g. The Gitrog Ride) keys off this flag.
//   - Vehicle → set "kw:artifact_creature" flag and grant creature type
//     until end of turn (engine-side type-grant pipeline reads this).
//   - Static "haste on Mounts and Vehicles you control" handled by the
//     AST keyword pipeline; emitPartial flags the boundary.
func registerKolodinTriumphCaster(r *Registry) {
	r.OnETB("Kolodin, Triumph Caster", kolodinTriumphCasterETB)
	r.OnTrigger("Kolodin, Triumph Caster", "permanent_etb", kolodinTriumphCasterETBTrigger)
	// End-of-turn sweep: clear the "until end of turn" flags Kolodin
	// stamped (saddled / artifact-creature-until-eot) and remove the
	// transient "creature_until_eot" type tag from vehicle Cards so
	// the type doesn't leak into next turn's combat. The engine's
	// generic cleanup pass *should* handle this, but the type-tag
	// slice mutation needs a card-level revert that the cleanup pass
	// doesn't (yet) drive — this sweep is defense-in-depth.
	r.OnTrigger("Kolodin, Triumph Caster", "end_step", kolodinTriumphCasterEOTSweep)
}

func kolodinTriumphCasterEOTSweep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kolodin_triumph_caster_eot_sweep"
	if gs == nil || perm == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	swept := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.Flags != nil {
			if p.Flags["saddled_until_eot"] != 0 {
				delete(p.Flags, "saddled")
				delete(p.Flags, "saddled_until_eot")
				swept++
			}
			if p.Flags["kw:artifact_creature_until_eot"] != 0 {
				delete(p.Flags, "kw:artifact_creature_until_eot")
				swept++
			}
		}
		// Strip the transient "creature_until_eot" tag if present.
		filtered := p.Card.Types[:0]
		removed := false
		for _, t := range p.Card.Types {
			if t == "creature_until_eot" {
				removed = true
				continue
			}
			filtered = append(filtered, t)
		}
		if removed {
			p.Card.Types = filtered
		}
	}
	if swept > 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":  perm.Controller,
			"swept": swept,
		})
	}
}

func kolodinTriumphCasterETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kolodin_triumph_caster_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	// R51 batch J: promote the haste static from AST-pipeline breadcrumb
	// to a real layer-6 continuous effect that grants kw:haste to every
	// Mount or Vehicle the controller controls. Predicate runs at
	// layer evaluation time so newly entering Mounts/Vehicles pick up
	// haste immediately, and the grant tears down via
	// UnregisterContinuousEffectsForPermanent on Kolodin's LTB.
	src := perm
	ts := perm.Timestamp
	suffix := strconv.Itoa(ts)
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Timestamp:      ts,
		SourcePerm:     src,
		SourceCardName: "Kolodin, Triumph Caster",
		ControllerSeat: perm.Controller,
		HandlerID:      "Kolodin, Triumph Caster:mount_vehicle_haste:" + suffix,
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate: func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
			if t == nil || t.Card == nil {
				return false
			}
			if t.Controller != src.Controller {
				return false
			}
			return cardSubtypeMatches(t.Card, "mount") || cardSubtypeMatches(t.Card, "vehicle")
		},
		ApplyFn: func(_ *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
			if chars != nil {
				already := false
				for _, k := range chars.Keywords {
					if k == "haste" {
						already = true
						break
					}
				}
				if !already {
					chars.Keywords = append(chars.Keywords, "haste")
				}
			}
			if target != nil {
				if target.Flags == nil {
					target.Flags = map[string]int{}
				}
				target.Flags["kw:haste"] = 1
			}
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":  perm.Controller,
		"layer": "6",
		"grant": "haste_to_mounts_vehicles",
	})
}

func kolodinTriumphCasterETBTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "kolodin_triumph_caster_mount_vehicle_etb"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered == perm || entered.Card == nil {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	if entered.Flags == nil {
		entered.Flags = map[string]int{}
	}
	if cardSubtypeMatches(entered.Card, "mount") {
		entered.Flags["saddled"] = 1
		// Track lifetime; cleared at end of turn by the cleanup pass.
		entered.Flags["saddled_until_eot"] = 1
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"target":   entered.Card.DisplayName(),
			"effect":   "saddled_until_eot",
		})
		return
	}
	if cardSubtypeMatches(entered.Card, "vehicle") {
		entered.Flags["kw:artifact_creature_until_eot"] = 1
		// Add a "creature" type tag for the duration so combat code sees
		// the vehicle as a creature without needing crew.
		entered.Card.Types = append(entered.Card.Types, "creature_until_eot")
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"target":   entered.Card.DisplayName(),
			"effect":   "becomes_artifact_creature_until_eot",
		})
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"vehicle_creature_type_grant_eot_cleanup_relies_on_engine_until_eot_pass")
		return
	}
}
