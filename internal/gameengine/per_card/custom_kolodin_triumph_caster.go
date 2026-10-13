package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerKolodinTriumphCasterCustom wires Kolodin's "Mounts and
// Vehicles you control have haste" anthem. The gen_*.go handler
// covers the per-ETB saddle / artifact-creature riders; its
// "_haste_static_handled_by_ast_keyword_pipeline" partial covers the
// haste anthem, which the AST keyword pipeline does NOT actually
// apply (Mount/Vehicle aren't first-class haste targets in the
// keyword pipeline). Stamping the flag here makes the haste real.
func registerKolodinTriumphCasterCustom(r *Registry) {
	r.OnETB("Kolodin, Triumph Caster", kolodinRefreshAnthemOnETB)
	r.OnTrigger("Kolodin, Triumph Caster", "permanent_etb", kolodinRefreshAnthemOnEvent)
	r.OnTrigger("Kolodin, Triumph Caster", "permanent_ltb", kolodinOwnLTB)
}

func kolodinRefreshAnthemOnETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	kolodinRefreshMountVehicleHaste(gs, perm)
}

func kolodinRefreshAnthemOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	kolodinRefreshMountVehicleHaste(gs, perm)
}

func kolodinRefreshMountVehicleHaste(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kolodin_mount_vehicle_haste_refresh"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Card == nil {
			continue
		}
		isMount := cardHasSubtype(p.Card, "mount")
		isVehicle := cardHasSubtype(p.Card, "vehicle") || cardHasType(p.Card, "vehicle")
		if !isMount && !isVehicle {
			continue
		}
		if p.Flags == nil {
			p.Flags = map[string]int{}
		}
		p.Flags["kw:haste"] = 1
		p.Flags["kw:haste_from_kolodin"] = 1
		stamped++
	}
	if stamped > 0 {
		gs.InvalidateCharacteristicsCache()
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":    perm.Controller,
			"stamped": stamped,
		})
	}
}

// ---------------------------------------------------------------------------
// r65 — static anthem stamps outlived their own source.
//
// The anthem is applied by stamping Flags["kw:<name>"] onto each
// matching creature, with a provenance marker recording who stamped it.
// The refresh function was registered for permanent_etb AND
// permanent_ltb, so when the ANTHEM SOURCE ITSELF left the battlefield
// the handler re-ran and RE-STAMPED instead of stripping: the buff
// survived its source for the rest of the game.
//
// The provenance marker existed precisely to make the strip possible;
// nothing ever performed it. Fixed by following the in-repo canonical
// shape (brudiclad_telchor_engineer.go): a dedicated own-LTB handler
// that compares ctx["perm"] against the source and strips the grants it
// owns, falling through to the ordinary recompute when some OTHER
// permanent is the one leaving.
// ---------------------------------------------------------------------------

func kolodinOwnLTB(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	if leaving, _ := ctx["perm"].(*gameengine.Permanent); leaving == perm {
		kolodinStripMountVehicleHaste(gs, perm)
		return
	}
	kolodinRefreshMountVehicleHaste(gs, perm)
}

// kolodinStripMountVehicleHaste drops only the haste grants this
// Kolodin stamped.
func kolodinStripMountVehicleHaste(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "kolodin_mount_vehicle_haste_strip"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	stripped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || p.Flags == nil {
			continue
		}
		if p.Flags["kw:haste_from_kolodin"] != 1 {
			continue
		}
		delete(p.Flags, "kw:haste_from_kolodin")
		delete(p.Flags, "kw:haste")
		stripped++
	}
	if stripped > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller, "stripped": stripped,
	})
}
