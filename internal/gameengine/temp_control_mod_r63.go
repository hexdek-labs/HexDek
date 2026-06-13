package gameengine

// temp_control_mod_r63.go — generic handler for the inert `temp_control`
// scaffold KIND (hex-dev-3, r63 scaffold-kind demolition).
//
// 39 cards parse the Threaten / Act of Treason template
//
//	"Gain control of target creature until end of turn. Untap that
//	 creature. It gains haste until end of turn."
//
// to a single Modification node with kind="temp_control" and EMPTY args.
// That kind had no case in resolveModificationEffect's switch, so it fell
// to the inert default (logged a parser_gap, did nothing) — Act of
// Treason, Goatnap, Portent of Betrayal, Traitorous Blood/Greed, Vengeful
// Possession, Unwilling Recruit, … were all no-ops.
//
// This mirrors the canonical resolveGainControl (resolve.go) for the
// until-end-of-turn case: change controller, register the steal for the
// cleanup-step revert (ExpireTempControlGrants, phases.go), untap, and
// grant the practical effect of haste (clear summoning sickness so the
// stolen creature can attack this turn). Ownership is untouched
// (CR §108.3). The steal reverts automatically at end of turn — no leak.
//
// Target selection: the node carries no target (empty args), so we pick
// the highest-power creature controlled by an opponent — the AI-optimal
// "target creature" for a one-turn steal-and-swing.
func resolveTempControlMod(gs *GameState, src *Permanent) {
	if gs == nil || src == nil {
		return
	}
	newController := controllerSeat(src)
	if newController < 0 || newController >= len(gs.Seats) {
		return
	}

	// Best steal: highest-power opponent creature.
	var best *Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(newController) {
		s := gs.Seats[opp]
		if s == nil {
			continue
		}
		for _, p := range s.Battlefield {
			if p == nil || p.PhasedOut || p.Controller == newController {
				continue
			}
			if !gs.IsCreatureOf(p) {
				continue
			}
			if pow := gs.PowerOf(p); pow > bestPow {
				bestPow = pow
				best = p
			}
		}
	}
	if best == nil {
		return
	}

	oldController := best.Controller
	gs.removePermanent(best)
	best.Controller = newController
	best.Timestamp = gs.NextTimestamp()
	gs.Seats[newController].Battlefield = append(gs.Seats[newController].Battlefield, best)

	// Until-EOT steal: register for the cleanup-step revert to owner.
	RegisterTempControlGrant(gs, best, oldController)

	// "Untap that creature. It gains haste until end of turn." — untap and
	// clear summoning sickness so it can attack this turn (the functional
	// haste). No persistent kw:haste flag is set: control reverts at end of
	// turn and a lingering flag would wrongly haste the owner's creature.
	best.Tapped = false
	best.SummoningSick = false

	gs.LogEvent(Event{
		Kind:   "gain_control",
		Seat:   newController,
		Target: oldController,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"target_card": best.Card.DisplayName(),
			"duration":    "until_end_of_turn",
			"via":         "temp_control",
		},
	})
}
