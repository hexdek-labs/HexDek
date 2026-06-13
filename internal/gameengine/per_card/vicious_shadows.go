package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerViciousShadows wires Vicious Shadows.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	Whenever a creature dies, you may have Vicious Shadows deal damage to target
//	player equal to the number of cards in that creature's controller's hand.
//
// Why a per-card handler: the trigger parses to an inert
// `parsed_effect_residual` ("you may have ... deal damage ... equal to ...
// hand") — generic dispatch deals no damage. An explosive aristocrats payoff.
//
// Implementation (CR §603 triggered ability):
//   - Listen on "creature_dies"; ctx "controller_seat" = the dead creature's
//     controller. Damage = that player's hand size.
//   - "you may ... target player": AI policy points it at the lowest-life
//     opponent of Vicious Shadows' controller (best reach). The optional clause
//     is taken whenever damage > 0 and a legal opponent exists.
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerViciousShadows(Global())
	AddResetHook(registerViciousShadows)
}

func registerViciousShadows(r *Registry) {
	r.OnTrigger("Vicious Shadows", "creature_dies", viciousShadowsOnDeath)
}

func viciousShadowsOnDeath(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vicious_shadows_death_damage"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadController, ok := ctx["controller_seat"].(int)
	if !ok || deadController < 0 || deadController >= len(gs.Seats) || gs.Seats[deadController] == nil {
		return
	}
	dmg := len(gs.Seats[deadController].Hand)
	if dmg <= 0 {
		return
	}

	// Target the lowest-life living opponent of Vicious Shadows' controller.
	target := -1
	for _, opp := range gs.Opponents(perm.Controller) {
		s := gs.Seats[opp]
		if s == nil || s.Lost {
			continue
		}
		if target < 0 || s.Life < gs.Seats[target].Life {
			target = opp
		}
	}
	if target < 0 {
		return
	}

	gameengine.DealDamage(gs, target, dmg, perm.Card.DisplayName())

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":           perm.Controller,
		"dead_controller": deadController,
		"target":         target,
		"damage":         dmg,
	})
}
