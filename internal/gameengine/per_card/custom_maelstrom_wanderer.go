package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerMaelstromWandererCustom replaces the auto-generated stub
// with a real implementation of Maelstrom Wanderer's two cascade
// triggers and creature-haste anthem.
//
// Oracle text:
//
//	Cascade, cascade (When you cast this spell, exile cards from the
//	  top of your library until you exile a nonland card that costs
//	  less. You may cast it without paying its mana cost. Put the
//	  exiled cards on the bottom in a random order. Then do it again.)
//	Creatures you control have haste.
//
// Implementation:
//   - Both cascade triggers fire from the engine's cast-path loop in
//     stack.go: CascadeCount() counts the two "cascade" keyword nodes
//     and ApplyCascade runs once per instance (CR §702.85c). This ETB
//     hook therefore does NOT fire cascade itself — doing so was a
//     triple-cascade bug once the CascadeCount loop landed (r63). It
//     only grants the haste anthem.
//   - The "creatures you control have haste" anthem is wired as a
//     layer-6 ContinuousEffect granting kw:haste to every creature
//     under Maelstrom's controller while she's on the battlefield.
func registerMaelstromWandererCustom(r *Registry) {
	r.OnETB("Maelstrom Wanderer", maelstromWandererETB)
}

func maelstromWandererETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "maelstrom_wanderer_etb"
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	source := perm
	seat := perm.Controller

	// Haste anthem. (Both cascades fire from the cast-path CascadeCount
	// loop in stack.go — CR §702.85c — not from here.)
	const grant = "haste"
	pred := func(_ *gameengine.GameState, t *gameengine.Permanent) bool {
		if t == nil || t.Card == nil {
			return false
		}
		if t.Controller != source.Controller {
			return false
		}
		return t.IsCreature()
	}
	apply := func(_ *gameengine.GameState, target *gameengine.Permanent, chars *gameengine.Characteristics) {
		if chars != nil {
			already := false
			for _, k := range chars.Keywords {
				if k == grant {
					already = true
					break
				}
			}
			if !already {
				chars.Keywords = append(chars.Keywords, grant)
			}
		}
		if target != nil {
			if target.Flags == nil {
				target.Flags = map[string]int{}
			}
			target.Flags["kw:haste"] = 1
			// Cancel summoning sickness so the haste grant has its
			// expected effect on creatures that ETB'd this turn.
			target.SummoningSick = false
		}
	}
	gs.RegisterContinuousEffect(&gameengine.ContinuousEffect{
		Layer:          gameengine.LayerAbility,
		Timestamp:      gs.NextTimestamp(),
		SourcePerm:     source,
		SourceCardName: "Maelstrom Wanderer",
		ControllerSeat: source.Controller,
		HandlerID:      "maelstrom_wanderer_haste_grant_" + perm.Card.DisplayName(),
		Duration:       gameengine.DurationUntilSourceLeaves,
		Predicate:      pred,
		ApplyFn:        apply,
	})
	// Stamp existing creatures now so summoning sickness clears
	// immediately for current-turn ETBs.
	for _, t := range gs.Seats[seat].Battlefield {
		if pred(gs, t) {
			apply(gs, t, nil)
		}
	}

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":          seat,
		"haste_anthem": true,
	})
}
