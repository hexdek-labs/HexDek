package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerZabazTheGlimmerwasp wires Zabaz, the Glimmerwasp.
//
// Oracle text:
//
//	Modular 1
//	If a modular triggered ability would put one or more +1/+1 counters
//	on a creature you control, that many plus one +1/+1 counters are put
//	on it instead.
//	{R}: Destroy target artifact you control.
//	{W}: Zabaz gains flying until end of turn.
//
// Implementation:
//   - Activated index 0 ({R}): destroy a target artifact we control.
//     Pick the lowest-CMC non-Zabaz artifact (utility sac fodder).
//   - Activated index 1 ({W}): grant flying until end of turn.
//   - Modular replacement effect (+1 to incoming +1/+1 from modular)
//     requires AST replacement plumbing; emitPartial.
func registerZabazTheGlimmerwasp(r *Registry) {
	r.OnActivated("Zabaz, the Glimmerwasp", zabazActivate)
}

func zabazActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil {
		return
	}
	switch abilityIdx {
	case 0:
		zabazDestroyArtifact(gs, src)
	case 1:
		zabazFlying(gs, src)
	default:
		emitPartial(gs, "zabaz_glimmerwasp_unknown_ability", src.Card.DisplayName(), "ability_index_oob")
	}
}

func zabazDestroyArtifact(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "zabaz_glimmerwasp_destroy_artifact"
	seat := gs.Seats[src.Controller]
	if seat == nil {
		return
	}
	var pick *gameengine.Permanent
	bestCMC := 1 << 30
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || p == src {
			continue
		}
		if !cardHasType(p.Card, "artifact") {
			continue
		}
		cmc := cardCMC(p.Card)
		if cmc < bestCMC {
			bestCMC = cmc
			pick = p
		}
	}
	if pick == nil {
		emitFail(gs, slug, src.Card.DisplayName(), "no_artifact_target", nil)
		return
	}
	destroyed := pick.Card.DisplayName()
	// DestroyPermanent handles §122.1b shield counters, §702.12b
	// indestructible, §614 would_die replacements, §903.9b commander
	// redirect (target artifact may be a commander artifact —
	// Hammer of Nazahn, Skullclamp commanders), detachAll, and LTB
	// triggers. Manual moveCardBetweenZones bypassed all of these.
	if gameengine.DestroyPermanent(gs, pick, src) {
		emitPartial(gs, "zabaz_modular_replacement", src.Card.DisplayName(),
			"modular_plus_one_replacement_not_implemented")
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      src.Controller,
			"destroyed": destroyed,
		})
	} else {
		emitFail(gs, slug, src.Card.DisplayName(), "destroy_failed_or_prevented", map[string]interface{}{
			"target": destroyed,
		})
	}
}

func zabazFlying(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "zabaz_glimmerwasp_flying"
	if src.Flags == nil {
		src.Flags = map[string]int{}
	}
	src.Flags["kw:flying"] = 1
	captured := src
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "next_end_step",
		ControllerSeat: src.Controller,
		SourceCardName: src.Card.DisplayName(),
		EffectFn: func(gs *gameengine.GameState) {
			if captured == nil || captured.Flags == nil {
				return
			}
			delete(captured.Flags, "kw:flying")
		},
	})
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat": src.Controller,
	})
}
