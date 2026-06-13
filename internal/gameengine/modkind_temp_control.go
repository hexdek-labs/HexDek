package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// modkind_temp_control.go — generic coverage for the `temp_control`
// scaffold ModKind: the Threaten / Act of Treason family —
//
//	Gain control of target creature until end of turn. Untap that
//	creature. It gains haste until end of turn.
//
// The `temp_control` kind fell through to the switch default and did
// NOTHING (39 cards: Act of Treason, Threaten, Goatnap, Unwilling
// Recruit, Portent of Betrayal, Kari Zev's Expertise, …), so these
// premier tempo/sacrifice-fodder spells stole nothing.
//
// The until-end-of-turn control machinery already exists
// (control_revert.go: RegisterTempControlGrant + ExpireTempControlGrants,
// the latter wired into the cleanup step at phases.go), so this handler
// performs the §674-style steal and registers the EOT revert. It mirrors
// the structured resolveGainControl path (which only fires for the proper
// GainControl AST node) and adds the untap + haste that the Threaten
// templating specifies.
//
// AI target policy: steal the opponents' highest-power creature (the
// biggest swing / best sacrifice fodder). No opponent creature → no-op.

func resolveTempControl(gs *GameState, src *Permanent, e *gameast.ModificationEffect) {
	if gs == nil || src == nil {
		return
	}
	seat := controllerSeat(src)
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}

	target := pickHighestPowerOpponentCreature(gs, seat)
	if target == nil {
		gs.LogEvent(Event{
			Kind:    "temp_control",
			Seat:    seat,
			Source:  sourceName(src),
			Details: map[string]interface{}{"stolen": false},
		})
		return
	}

	oldController := target.Controller

	// Control change (CR §108.3: owner never changes). Move to the
	// caster's battlefield and register the until-EOT revert so the
	// cleanup step (§514.2) returns it to its owner.
	gs.removePermanent(target)
	target.Controller = seat
	target.Timestamp = gs.NextTimestamp()
	gs.Seats[seat].Battlefield = append(gs.Seats[seat].Battlefield, target)
	RegisterTempControlGrant(gs, target, oldController)

	// "Untap that creature and it gains haste until end of turn."
	target.Tapped = false
	target.SummoningSick = false
	if !target.HasKeyword("haste") {
		target.GrantedAbilities = append(target.GrantedAbilities, "haste")
	}

	gs.LogEvent(Event{
		Kind:   "temp_control",
		Seat:   seat,
		Target: oldController,
		Source: sourceName(src),
		Details: map[string]interface{}{
			"stolen":      true,
			"target_card": target.Card.DisplayName(),
			"duration":    "until_end_of_turn",
		},
	})
}

// pickHighestPowerOpponentCreature returns the highest-power creature any
// opponent of seat controls (the best Threaten target), or nil.
func pickHighestPowerOpponentCreature(gs *GameState, seat int) *Permanent {
	var best *Permanent
	bestPow := -1
	for _, opp := range gs.Opponents(seat) {
		if opp < 0 || opp >= len(gs.Seats) || gs.Seats[opp] == nil {
			continue
		}
		for _, p := range gs.Seats[opp].Battlefield {
			if p == nil || p.Card == nil || !p.IsCreature() {
				continue
			}
			if pw := p.Power(); pw > bestPow {
				bestPow = pw
				best = p
			}
		}
	}
	return best
}
