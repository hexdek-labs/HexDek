package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// inspired_ultimatum_r60.go — per_card handler for Inspired Ultimatum.
//
// Oracle text (Scryfall / ast_dataset):
//
//	Target player gains 5 life, Inspired Ultimatum deals 5 damage to any
//	target, then you draw five cards.
//
// {U}{U}{R}{R}{R}{W}{W} Sorcery. A Jeskai "ultimatum" payoff (life +
// reach + a five-card refill). Parses to an inert
// `parsed_effect_residual` node with no structured life/damage/draw
// nodes, so the whole spell did NOTHING. Demonstrates gameengine.DrawN
// composing with the existing GainLife / DealDamage primitives.
//
// Implementation:
//   - OnResolve. Hat policy: the "target player gains 5 life" goes to
//     the caster (dominant line — no reason to pad an opponent); the
//     "5 damage to any target" goes to the highest-life opponent (a
//     player target, the canonical pick when no creature target was
//     stamped); then the caster draws five via DrawN.
func init() {
	registerInspiredUltimatumR60(Global())
	AddResetHook(registerInspiredUltimatumR60)
}

func registerInspiredUltimatumR60(r *Registry) {
	if r == nil {
		return
	}
	r.OnResolve("Inspired Ultimatum", inspiredUltimatumResolve)
}

func inspiredUltimatumResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "inspired_ultimatum"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return
	}

	// Caster gains 5 life.
	gameengine.GainLife(gs, seat, 5, "Inspired Ultimatum")

	// 5 damage to the highest-life opponent.
	target := -1
	bestLife := -1
	for _, opp := range gs.Opponents(seat) {
		os := gs.Seats[opp]
		if os == nil || os.Lost {
			continue
		}
		if os.Life > bestLife {
			bestLife = os.Life
			target = opp
		}
	}
	if target >= 0 {
		gameengine.DealDamage(gs, target, 5, "Inspired Ultimatum")
	}

	// Caster draws five.
	var src *gameengine.Permanent
	if item.Card != nil {
		src = &gameengine.Permanent{Card: item.Card, Controller: seat, Owner: item.Card.Owner}
	}
	drawn := gameengine.DrawN(gs, seat, 5, src)

	emit(gs, slug, "Inspired Ultimatum", map[string]interface{}{
		"seat":         seat,
		"damaged_seat": target,
		"drawn":        drawn,
	})
}
