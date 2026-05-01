package per_card

import (
	"math/rand"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerVialSmasher wires Vial Smasher the Fierce.
//
// Oracle text:
//
//	Whenever you cast your first spell each turn, choose an opponent at
//	random. Vial Smasher deals damage to that player equal to that spell's
//	mana value.
//	Partner
//
// Listens on "spell_cast" and tracks whether the first spell this turn
// has already fired via a per-turn flag.
func registerVialSmasher(r *Registry) {
	r.OnTrigger("Vial Smasher the Fierce", "spell_cast", vialSmasherTrigger)
}

func vialSmasherTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "vial_smasher_first_spell_damage"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	casterSeat := -1
	if v, ok := ctx["seat"].(int); ok {
		casterSeat = v
	}
	if casterSeat != perm.Controller {
		return
	}

	flagKey := "vial_smasher_fired"
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags[flagKey] >= gs.Turn {
		return
	}
	perm.Flags[flagKey] = gs.Turn

	mv := 0
	if c, ok := ctx["card"].(*gameengine.Card); ok && c != nil {
		mv = gameengine.ManaCostOf(c)
	}
	if mv <= 0 {
		return
	}

	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	target := opps[rand.Intn(len(opps))]
	opp := gs.Seats[target]
	if opp == nil || opp.Lost {
		return
	}
	opp.Life -= mv
	gs.LogEvent(gameengine.Event{
		Kind:   "damage",
		Seat:   perm.Controller,
		Target: target,
		Source: "Vial Smasher the Fierce",
		Amount: mv,
		Details: map[string]interface{}{
			"slug":       slug,
			"mana_value": mv,
			"random":     true,
		},
	})

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        perm.Controller,
		"target_seat": target,
		"damage":      mv,
	})
}
