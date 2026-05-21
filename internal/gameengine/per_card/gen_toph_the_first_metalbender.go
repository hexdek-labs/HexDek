package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTophTheFirstMetalbender wires Toph, the First Metalbender.
//
// Oracle text:
//
//	Nontoken artifacts you control are lands in addition to their
//	other types. (They don't gain the ability to {T} for mana.)
//	At the beginning of your end step, earthbend 2. (Target land you
//	control becomes a 0/0 creature with haste that's still a land.
//	Put two +1/+1 counters on it. When it dies or is exiled, return
//	it to the battlefield tapped.)
//
// Implementation (R46 stub port):
//   - OnTrigger("end_step") gated on active_seat == controller fires
//     earthbend 2 against a chosen non-creature land we control.
//     The chosen land gets `earthbent`, `temp_haste`, and 2x +1/+1
//     counters — same shape as Toph, Earthbending Master's
//     `tophEarthbend` helper.
//   - The static "non-token artifacts you control are lands" type-add
//     is a layer-4 add safely handled by the AST keyword pipeline for
//     lord-shape statics; remains partial.
//   - The "return when it dies/is exiled" rider on the earthbent land
//     is a per-target LTB replacement not exposed here — partial.
func registerTophTheFirstMetalbender(r *Registry) {
	r.OnETB("Toph, the First Metalbender", tophFirstMetalbenderETB)
	r.OnTrigger("Toph, the First Metalbender", "end_step", tophFirstMetalbenderEndStep)
	// R55: nontoken artifacts you control are lands — Layer 4 add-types
	// stamped on each qualifying artifact at ETB + on permanent_etb
	// refresh. Note the artifacts gain the "land" subtype but do NOT
	// gain a tap-for-mana ability (that's a printed clarification, not
	// a layered effect — they only have the subtype).
	r.OnTrigger("Toph, the First Metalbender", "permanent_etb", tophFirstMetalbenderRefreshLandGrants)
}

// tophFirstMetalbenderStampLandTypes walks the controller's
// battlefield and registers a Layer-4 add-types effect adding "land"
// to every nontoken artifact. Idempotent via the source perm + tag.
func tophFirstMetalbenderStampLandTypes(gs *gameengine.GameState, perm *gameengine.Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return 0
	}
	stamped := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if !p.IsArtifact() {
			continue
		}
		// Skip token artifacts.
		if cardHasType(p.Card, "token") {
			continue
		}
		gameengine.RegisterAddTypes(gs, p, []string{"land"},
			gameengine.DurationPermanent,
			"Toph, the First Metalbender",
			"toph_1stmb_artifact_is_land")
		stamped++
	}
	return stamped
}

func tophFirstMetalbenderRefreshLandGrants(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toph_first_metalbender_refresh_land_grant"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered.Card == nil {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	if !entered.IsArtifact() {
		return
	}
	if cardHasType(entered.Card, "token") {
		return
	}
	gameengine.RegisterAddTypes(gs, entered, []string{"land"},
		gameengine.DurationPermanent,
		"Toph, the First Metalbender",
		"toph_1stmb_artifact_is_land")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"artifact": entered.Card.DisplayName(),
	})
}

func tophFirstMetalbenderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_first_metalbender_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	stamped := tophFirstMetalbenderStampLandTypes(gs, perm)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":              perm.Controller,
		"artifacts_stamped": stamped,
	})
}

func tophFirstMetalbenderEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "toph_first_metalbender_earthbend"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	// Pick a non-creature land we control.
	var target *gameengine.Permanent
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if p.IsLand() && !p.IsCreature() {
			target = p
			break
		}
	}
	if target == nil {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":   perm.Controller,
			"reason": "no_land_target",
		})
		return
	}
	if target.Flags == nil {
		target.Flags = map[string]int{}
	}
	target.Flags["earthbent"] = 1
	// R54 layer-7b port: earthbend 2 — Layer 4 (creature) + Layer 7b
	// (0/0) + Layer 6 (haste), all keyed to the target so they tear
	// down on LTB. 2× +1/+1 counters stack via §613.4c.
	gameengine.RegisterAddTypes(gs, target, []string{"creature"},
		gameengine.DurationUntilSourceLeaves, "Toph, the First Metalbender", "earthbend")
	gameengine.RegisterSetPT(gs, target, 0, 0,
		gameengine.DurationUntilSourceLeaves, "Toph, the First Metalbender", "earthbend")
	gameengine.RegisterGrantKeyword(gs, target, "haste",
		gameengine.DurationUntilSourceLeaves, "Toph, the First Metalbender", "earthbend")
	target.AddCounter("+1/+1", 2)
	target.SummoningSick = false
	gs.InvalidateCharacteristicsCache()
	gameengine.Earthbend(gs, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"target":   target.Card.DisplayName(),
		"counters": 2,
		"layer_7b": [2]int{0, 0},
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"return_on_die_or_exile_replacement_partial")
}
