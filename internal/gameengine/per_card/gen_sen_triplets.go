package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSenTriplets wires Sen Triplets.
//
// Oracle text:
//
//	At the beginning of your upkeep, choose target opponent. This turn,
//	that player can't cast spells or activate abilities and plays with
//	their hand revealed. You may play lands and cast spells from that
//	player's hand this turn.
//
// Implementation:
//   - "upkeep_controller" trigger gated on the controller. Picks the
//     highest-life living opponent (most threatening — biggest hand
//     usually correlates) as the target.
//   - Stamps three flags on the target opponent for the engine to honor:
//       "sen_triplets_locked"   — can't cast spells or activate abilities
//       "sen_triplets_revealed" — plays with hand revealed
//     And one flag on the controller:
//       "sen_triplets_play_from"  = target_seat + 1
//   - All three flags are cleared at end of turn via a delayed trigger.
//   - Restriction enforcement (cast/activate gates, hand-visibility,
//     play-from-other-hand pipeline) lives at the engine layer; we
//     surface partials so audits can find the wiring boundary.
func registerSenTriplets(r *Registry) {
	r.OnTrigger("Sen Triplets", "upkeep_controller", senTripletsUpkeep)
	// R51 batch I: LTB sweeps the per-opponent locked/revealed flags +
	// the controller's play_from flag if Sen left mid-turn before the
	// delayed end-of-turn cleanup runs. Without this, killing Sen
	// during the upkeep-pickup turn leaves the lockout stuck on the
	// targeted opponent for the rest of the turn.
	r.OnTrigger("Sen Triplets", "permanent_ltb", senTripletsLTBSweep)
}

func senTripletsLTBSweep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	leaving, _ := ctx["perm"].(*gameengine.Permanent)
	if leaving != perm {
		return
	}
	for _, s := range gs.Seats {
		if s == nil || s.Flags == nil {
			continue
		}
		delete(s.Flags, "sen_triplets_locked")
		delete(s.Flags, "sen_triplets_revealed")
	}
	ctl := gs.Seats[perm.Controller]
	if ctl != nil && ctl.Flags != nil {
		delete(ctl.Flags, "sen_triplets_play_from")
	}
	// R57: drop the cast-from-opp-hand ZoneCastPolicy if Sen leaves
	// mid-turn before the delayed end-of-turn cleanup runs.
	gs.UnregisterZoneCastPoliciesForPermanent(perm)
}

func senTripletsUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sen_triplets_upkeep_lock"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	// Target = highest-life living opponent.
	target := -1
	bestLife := -1 << 30
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if s.Life > bestLife {
			bestLife = s.Life
			target = opp
		}
	}
	if target < 0 {
		return
	}
	// Stamp flags on target.
	tSeat := gs.Seats[target]
	if tSeat == nil {
		return
	}
	if tSeat.Flags == nil {
		tSeat.Flags = map[string]int{}
	}
	tSeat.Flags["sen_triplets_locked"] = 1
	tSeat.Flags["sen_triplets_revealed"] = 1
	// Stamp on controller.
	ctlSeat := gs.Seats[perm.Controller]
	if ctlSeat != nil {
		if ctlSeat.Flags == nil {
			ctlSeat.Flags = map[string]int{}
		}
		ctlSeat.Flags["sen_triplets_play_from"] = target + 1 // +1 to allow 0-seat encoding
	}

	// R57: register a ZoneCastPolicy granting cast-from-opponent's-hand.
	// OwnerScope=opponents + CasterScope=controller routes Sen's
	// printed "cast spells from that player's hand" through the R55
	// alt-cost cast pipeline rather than the breadcrumb-only flag
	// set above (the flag stays for back-compat with any sibling
	// code that already reads it).
	gs.RegisterZoneCastPolicy(&gameengine.ZoneCastPolicy{
		SourcePerm:      perm,
		HandlerID:       "sen_triplets_cast_from_opp_hand",
		Zone:            gameengine.ZoneHand,
		OwnerScope:      "opponents",
		CasterScope:     "controller",
		ControllerSeat:  perm.Controller,
		ManaCost:        -1, // pay the card's printed cost
		SpendAnyColor:   true,
		Duration:        "until_end_of_turn",
		SourceTimestamp: perm.Timestamp,
		GrantTurn:       gs.Turn,
	})

	// Cleanup at end of turn.
	gs.RegisterDelayedTrigger(&gameengine.DelayedTrigger{
		TriggerAt:      "end_of_turn",
		ControllerSeat: perm.Controller,
		SourceCardName: perm.Card.DisplayName(),
		OneShot:        true,
		EffectFn: func(gs *gameengine.GameState) {
			if ts := gs.Seats[target]; ts != nil && ts.Flags != nil {
				delete(ts.Flags, "sen_triplets_locked")
				delete(ts.Flags, "sen_triplets_revealed")
			}
			if cs := gs.Seats[perm.Controller]; cs != nil && cs.Flags != nil {
				delete(cs.Flags, "sen_triplets_play_from")
			}
			gs.UnregisterZoneCastPoliciesForPermanent(perm)
		},
	})
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_seat": target,
		"policy":      "sen_triplets_cast_from_opp_hand",
	})
	// R57: the cast lock + hand reveal are still engine-deep gates
	// (seat flags surfaced for the engine's cast-legality / opp-hand-
	// reveal pipeline to honor when those surfaces land). The
	// play-from-opponent's-hand half is now wired via ZoneCastPolicy.
}
