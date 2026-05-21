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
}

func tophFirstMetalbenderETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "toph_first_metalbender_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": perm.Controller,
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"nontoken_artifacts_become_lands_static_handled_by_ast_keyword_pipeline")
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
