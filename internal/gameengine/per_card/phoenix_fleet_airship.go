package per_card

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerPhoenixFleetAirship wires Phoenix Fleet Airship.
//
// Oracle text (Scryfall, verified 2026-05-16 via hexdek.dev oracle):
//
//	Flying
//	At the beginning of your end step, if you sacrificed a permanent
//	this turn, create a token that's a copy of this Vehicle.
//	As long as you control eight or more permanents named Phoenix Fleet
//	Airship, this Vehicle is an artifact creature.
//	Crew 1
//
// Implementation (Muninn gap #34 — 28K hits):
//   - Flying / Crew handled by AST keyword pipeline.
//   - OnTrigger("end_step") gated on controller == active seat and on
//     seat.Turn.Sacrificed > 0 (state.go's TurnCounters.Sacrificed).
//     Token copy via Card.DeepCopy + enterBattlefieldWithETB mirrors
//     resolve.go:resolveCreateTokenCopy (line 1755).
//   - The "becomes a creature when you control 8+" static type-changing
//     overlay needs the Phase 8 layers pass. emitPartial.
func registerPhoenixFleetAirship(r *Registry) {
	r.OnTrigger("Phoenix Fleet Airship", "end_step", phoenixFleetAirshipEndStep)
	// R55: 8+ named-copies → artifact creature via Layer 4 add-types.
	// Refresh on ETB (self + each Airship token spawn) and on
	// permanent_ltb (a copy dying may drop us below 8).
	r.OnETB("Phoenix Fleet Airship", phoenixFleetAirshipCheckThreshold)
	r.OnTrigger("Phoenix Fleet Airship", "permanent_etb", phoenixFleetAirshipRefreshOnEvent)
	r.OnTrigger("Phoenix Fleet Airship", "permanent_ltb", phoenixFleetAirshipRefreshOnEvent)
}

func phoenixFleetAirshipRefreshOnEvent(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	phoenixFleetAirshipCheckThreshold(gs, perm)
}

// phoenixFleetAirshipCheckThreshold registers (or skips) a Layer 4
// add-creature-type effect when controller has 8+ Airship copies.
// The "as long as" clause is sticky-while-condition-holds; we model
// it as DurationUntilSourceLeaves on this perm. Multi-perm boards
// each get their own registration with idempotency via the
// phoenix_fleet_creature_active flag.
func phoenixFleetAirshipCheckThreshold(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil || perm.Card == nil {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil {
			continue
		}
		if strings.EqualFold(p.Card.DisplayName(), "Phoenix Fleet Airship") {
			count++
		}
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	hasGrant := perm.Flags["phoenix_fleet_creature_active"] == 1
	wantGrant := count >= 8
	if hasGrant == wantGrant {
		return
	}
	if wantGrant {
		gameengine.RegisterAddTypes(gs, perm, []string{"creature"},
			gameengine.DurationUntilSourceLeaves,
			"Phoenix Fleet Airship (8+ named copies)",
			"phoenix_fleet_creature_active")
		perm.Flags["phoenix_fleet_creature_active"] = 1
	} else {
		// Drop count; unregister our Layer 4 effect. The continuous
		// effects framework supports per-permanent unregistration via
		// the source pointer + HandlerID match.
		gs.UnregisterContinuousEffectsForPermanent(perm)
		delete(perm.Flags, "phoenix_fleet_creature_active")
	}
	emit(gs, "phoenix_fleet_airship_threshold_check", perm.Card.DisplayName(), map[string]interface{}{
		"seat":         perm.Controller,
		"count":        count,
		"creature_now": wantGrant,
	})
}

func phoenixFleetAirshipEndStep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "phoenix_fleet_airship_copy_on_sacrifice"
	if gs == nil || perm == nil || ctx == nil || perm.Card == nil {
		return
	}
	activeSeat, _ := ctx["active_seat"].(int)
	if activeSeat != perm.Controller {
		return
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}
	if seat.Turn.Sacrificed <= 0 {
		emitFail(gs, slug, perm.Card.DisplayName(), "no_permanent_sacrificed_this_turn", map[string]interface{}{
			"seat": seatIdx,
		})
		return
	}
	card := perm.Card.DeepCopy()
	hasToken := false
	for _, t := range card.Types {
		if strings.EqualFold(t, "token") {
			hasToken = true
			break
		}
	}
	if !hasToken {
		card.Types = append([]string{"token"}, card.Types...)
	}
	card.Owner = seatIdx
	token := enterBattlefieldWithETB(gs, seatIdx, card, false)
	if token == nil {
		emitFail(gs, slug, perm.Card.DisplayName(), "token_creation_failed", nil)
		return
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":       seatIdx,
		"sacrificed": seat.Turn.Sacrificed,
		"token":      "Phoenix Fleet Airship (token copy)",
	})
	// R55: the new token's ETB will fire phoenixFleetAirshipCheckThreshold
	// for every existing Airship on the controller's battlefield via the
	// permanent_etb trigger; that pass re-evaluates the 8+ count.
}
