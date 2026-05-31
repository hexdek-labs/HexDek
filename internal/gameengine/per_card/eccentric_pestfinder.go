package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerEccentricPestfinder wires Eccentric Pestfinder // Turn Stones
// (Muninn parser-gap #103, ~1.5K hits).
//
// Oracle text (Scryfall, verified 2026-05-17):
//
//	Eccentric Pestfinder — {2}{B}{G} Creature — Troll Druid 4/3
//	Trample
//	At the beginning of each end step, if you gained life this turn,
//	this creature becomes prepared. (While it's prepared, you may cast
//	a copy of its spell. Doing so unprepares it.)
//
//	Turn Stones (back) — {B}{G} Sorcery
//	For each opponent, you create a 1/1 black and green Pest creature
//	token with "When this token dies, you gain 1 life."
//
// Implementation:
//   - end_step: if active_seat == controller and Turn.LifeGained > 0,
//     stamp prepared. The cast-copy mechanic is partial.
//   - The back face is a sorcery, not a permanent — its resolution path
//     belongs in resolve.go, not per_card battlefield hooks. We attach a
//     resolve handler that mints one Pest token per opponent so the
//     parser-gap actually clears when Turn Stones lands.
func registerEccentricPestfinder(r *Registry) {
	r.OnTrigger("Eccentric Pestfinder", "end_step", eccentricPestfinderEndStep)
	r.OnResolve("Turn Stones", turnStonesResolve)
	// R60 batch 8: wire the printed-on-token "When this token dies,
	// you gain 1 life" trigger. Pest tokens minted via Turn Stones
	// carry Flags["pestfinder_die_lifegain"]=1 stamped at mint so the
	// generic "Pest Token" creature_dies handler only fires for
	// Turn-Stones-spawned Pests (not Lluwen-spawned Pests, which use
	// pest_attack_lifegain from batch 5).
	r.OnTrigger("Pest Token", "creature_dies", pestfinderPestDiesLifegain)
}

func pestfinderPestDiesLifegain(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "pestfinder_pest_dies_lifegain"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	// creature_dies handler fires once per battlefield Pest; only act
	// when the dying perm IS this Pest, not when SOME OTHER Pest dies
	// (avoids double-trigger on simultaneous deaths).
	dying, _ := ctx["perm"].(*gameengine.Permanent)
	if dying != perm {
		return
	}
	if perm.Flags == nil || perm.Flags["pestfinder_die_lifegain"] != 1 {
		// Not a Pestfinder-spawned Pest (e.g. Lluwen's Pest, or some
		// other source) — the printed-on-token trigger doesn't apply.
		return
	}
	gameengine.GainLife(gs, perm.Controller, 1, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"gained": 1,
	})
}

func eccentricPestfinderEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "eccentric_pestfinder_end_step"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	gained := gs.Seats[perm.Controller].Turn.LifeGained
	if gained <= 0 {
		emit(gs, slug, "Eccentric Pestfinder", map[string]interface{}{
			"seat":      perm.Controller,
			"triggered": false,
			"gained":    gained,
		})
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["prepared"] = 1
	emit(gs, slug, "Eccentric Pestfinder", map[string]interface{}{
		"seat":      perm.Controller,
		"triggered": true,
		"gained":    gained,
	})
	emitPartial(gs, slug, "Eccentric Pestfinder",
		"prepared_cast_copy_of_back_face_turn_stones_requires_cast_pipeline_hook")
}

func turnStonesResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "turn_stones_resolve"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	opps := gs.Opponents(seat)
	for range opps {
		tok := gameengine.CreateCreatureToken(gs, seat, "Pest Token",
			[]string{"creature", "pest"}, 1, 1)
		if tok != nil {
			if tok.Card != nil {
				tok.Card.Colors = []string{"B", "G"}
			}
			if tok.Flags == nil {
				tok.Flags = map[string]int{}
			}
			tok.Flags["pestfinder_die_lifegain"] = 1
		}
	}
	emit(gs, slug, "Turn Stones", map[string]interface{}{
		"seat":     seat,
		"opponent": len(opps),
		"tokens":   len(opps),
	})
}
