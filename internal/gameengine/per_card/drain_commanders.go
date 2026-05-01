package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Drain commanders — lifegain-to-drain and ETB-drain triggers that
// were missing from the per-card registry. These are the core cards
// for the aristocrats/drain archetype identified in the bottom-40
// ELO analysis.

// --- Lifegain → drain ---

func registerDinaSoulSteeper(r *Registry) {
	r.OnTrigger("Dina, Soul Steeper", "life_gained", dinaDrainTrigger)
}

func registerDinaEssenceBrewer(r *Registry) {
	r.OnTrigger("Dina, Essence Brewer", "life_gained", dinaDrainTrigger)
}

func dinaDrainTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	gainSeat := -1
	if v, ok := ctx["seat"].(int); ok {
		gainSeat = v
	}
	if gainSeat != perm.Controller {
		return
	}
	for _, opp := range gs.Opponents(perm.Controller) {
		if gs.Seats[opp] == nil || gs.Seats[opp].Lost {
			continue
		}
		gs.Seats[opp].Life -= 1
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   opp,
			Source: perm.Card.DisplayName(),
			Amount: 1,
		})
	}
	emit(gs, "dina_drain", perm.Card.DisplayName(), map[string]interface{}{
		"seat":       perm.Controller,
		"drain_each": 1,
	})
}

func registerVitoThornOfTheDuskRose(r *Registry) {
	r.OnTrigger("Vito, Thorn of the Dusk Rose", "life_gained", vitoDrainTrigger)
}

func registerVitoFanaticOfAclazotz(r *Registry) {
	r.OnTrigger("Vito, Fanatic of Aclazotz", "life_gained", vitoDrainTrigger)
}

func vitoDrainTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	gainSeat := -1
	if v, ok := ctx["seat"].(int); ok {
		gainSeat = v
	}
	if gainSeat != perm.Controller {
		return
	}
	amount := 0
	if v, ok := ctx["amount"].(int); ok {
		amount = v
	}
	if amount <= 0 {
		return
	}
	opps := gs.Opponents(perm.Controller)
	if len(opps) == 0 {
		return
	}
	bestOpp := opps[0]
	bestLife := gs.Seats[opps[0]].Life
	for _, opp := range opps[1:] {
		if gs.Seats[opp].Life > bestLife {
			bestLife = gs.Seats[opp].Life
			bestOpp = opp
		}
	}
	gs.Seats[bestOpp].Life -= amount
	gs.LogEvent(gameengine.Event{
		Kind:   "lose_life",
		Seat:   bestOpp,
		Source: perm.Card.DisplayName(),
		Amount: amount,
	})
	emit(gs, "vito_drain", perm.Card.DisplayName(), map[string]interface{}{
		"seat":      perm.Controller,
		"target":    bestOpp,
		"life_lost": amount,
	})
}

func registerMaraudingBlightPriest(r *Registry) {
	r.OnTrigger("Marauding Blight-Priest", "life_gained", dinaDrainTrigger)
}

// --- ETB → drain ---

func registerCorpseKnight(r *Registry) {
	r.OnTrigger("Corpse Knight", "nonland_permanent_etb", corpseKnightTrigger)
}

func corpseKnightTrigger(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	if gs == nil || perm == nil {
		return
	}
	enterSeat := -1
	if v, ok := ctx["controller_seat"].(int); ok {
		enterSeat = v
	}
	if enterSeat != perm.Controller {
		return
	}
	// Check that the entering permanent is a creature.
	if enteredCard, ok := ctx["card"].(*gameengine.Card); ok && enteredCard != nil {
		isCreature := false
		for _, t := range enteredCard.Types {
			if t == "creature" {
				isCreature = true
				break
			}
		}
		if !isCreature {
			return
		}
	}
	for _, opp := range gs.Opponents(perm.Controller) {
		if gs.Seats[opp] == nil || gs.Seats[opp].Lost {
			continue
		}
		gs.Seats[opp].Life -= 1
		gs.LogEvent(gameengine.Event{
			Kind:   "lose_life",
			Seat:   opp,
			Source: "Corpse Knight",
			Amount: 1,
		})
	}
	emit(gs, "corpse_knight_drain", "Corpse Knight", map[string]interface{}{
		"seat":       perm.Controller,
		"drain_each": 1,
	})
}
