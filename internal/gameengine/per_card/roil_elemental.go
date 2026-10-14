package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerRoilElemental wires Roil Elemental (Muninn parser-gap rank ~133,
// landfall control-magic engine).
//
// Oracle text (Scryfall, verified 2026-05-17 via hexdek.dev oracle):
//
//	{3}{U}{U}{U}
//	Creature — Elemental
//	Flying
//	Landfall — Whenever a land you control enters, you may gain control
//	of target creature for as long as you control this creature.
//
// Implementation:
//   - Flying via AST keyword pipeline.
//   - OnTrigger("permanent_etb"): gate on entering perm being a land
//     controlled by perm.Controller (the standard landfall pattern, see
//     custom_choco_seeker_of_paradise.go).
//   - Effect: pick the best opponent creature (highest power, then
//     highest CMC) and reassign Controller to perm.Controller. The
//     "for as long as you control Roil Elemental" duration is not yet
//     modelled in the engine — the control change persists past
//     Roil's removal — so we emitPartial about that. Cards we steal
//     will be moved back to their owner at LTB via §403 if/when
//     gain-control duration tracking ships.
func registerRoilElemental(r *Registry) {
	r.OnTrigger("Roil Elemental", "permanent_etb", roilElementalLandfall)
}

func roilElementalLandfall(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "roil_elemental_landfall_steal"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	entered, _ := ctx["perm"].(*gameengine.Permanent)
	if entered == nil || entered.Card == nil || !entered.IsLand() {
		return
	}
	if entered.Controller != perm.Controller {
		return
	}
	// Pick best opponent creature.
	var target *gameengine.Permanent
	bestScore := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			score := p.Power()*10 + cardCMC(p.Card)
			if score > bestScore {
				bestScore = score
				target = p
			}
		}
	}
	if target == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_opponent_creature", map[string]interface{}{
			"seat": perm.Controller,
		})
		return
	}
	oldController := target.Controller
	oldSeat := gs.Seats[oldController]
	newSeat := gs.Seats[perm.Controller]
	if oldSeat == nil || newSeat == nil {
		return
	}
	// r102 — canonical reparent (CR §302.6 stamp, §702.95e unpair, §613
	// invalidation). Roil Elemental's landfall steal prints no haste.
	gameengine.ApplyControlChange(gs, target, perm.Controller)
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"stolen":         target.Card.DisplayName(),
		"from_seat":      oldController,
		"landfall_from":  entered.Card.DisplayName(),
	})
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"control_duration_tied_to_roil_presence_not_yet_modelled_steal_is_permanent")
}
