package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// dictate_of_erebos.go — per_card handler for Dictate of Erebos.
//
// Oracle text (Enchantment, {3}{B}{B}):
//
//	Flash
//	Whenever a creature you control dies, each opponent sacrifices a
//	creature of their choice.
//
// Why this needs a per_card handler: the trigger parsed to inert
// scaffold, so this aristocrats edict engine (and its twin Grave Pact)
// never made opponents sacrifice. Verified inert: no registered handler
// for "Dictate of Erebos".
//
// Implemented here (OnTrigger creature_dies): when a creature you control
// dies, each opponent sacrifices a creature. "Of their choice" → AI
// policy sacrifices their least valuable creature.

func init() {
	registerDictateOfErebos(Global())
	AddResetHook(registerDictateOfErebos)
}

func registerDictateOfErebos(r *Registry) {
	if r == nil {
		return
	}
	r.OnTrigger("Dictate of Erebos", "creature_dies", dictateOfErebosDies)
}

func dictateOfErebosDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "dictate_of_erebos"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	deadSeat, ok := ctx["controller_seat"].(int)
	if !ok || deadSeat != perm.Controller {
		return // only a creature YOU control dying triggers the edict
	}
	for _, opp := range gs.Opponents(perm.Controller) {
		victim := weakestCreatureOf(gs, opp)
		if victim == nil {
			continue
		}
		name := victim.Card.DisplayName()
		gameengine.SacrificePermanent(gs, victim, "Dictate of Erebos")
		emit(gs, slug, "Dictate of Erebos", map[string]interface{}{
			"seat":       perm.Controller,
			"opponent":   opp,
			"sacrificed": name,
		})
	}
	_ = gs.CheckEnd()
}

// weakestCreatureOf returns the lowest-power creature the seat controls
// (the one an AI opponent would most willingly sacrifice to an edict),
// or nil if they control no creatures.
func weakestCreatureOf(gs *gameengine.GameState, seat int) *gameengine.Permanent {
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		return nil
	}
	var worst *gameengine.Permanent
	worstPow := 1 << 30
	for _, p := range gs.Seats[seat].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if pw := p.Power(); pw < worstPow {
			worstPow = pw
			worst = p
		}
	}
	return worst
}
