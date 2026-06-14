package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAerithGainsborough wires Aerith Gainsborough.
//
// Oracle text:
//
//	Lifelink
//	Whenever you gain life, put a +1/+1 counter on Aerith Gainsborough.
//	When Aerith Gainsborough dies, put X +1/+1 counters on each
//	legendary creature you control, where X is the number of +1/+1
//	counters on Aerith Gainsborough.
func registerAerithGainsborough(r *Registry) {
	r.OnTrigger("Aerith Gainsborough", "life_gained", aerithLifeGained)
	r.OnTrigger("Aerith Gainsborough", "dies", aerithDies)
}

func aerithLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aerith_life_gained_counter"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	seat, _ := ctx["seat"].(int)
	if seat != perm.Controller {
		return
	}
	amount, _ := ctx["amount"].(int)
	if amount <= 0 {
		return
	}
	perm.AddCounter("+1/+1", 1)
	gs.InvalidateCharacteristicsCache()
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"amount": amount,
	})
}

func aerithDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "aerith_death_distribute"
	if gs == nil || perm == nil {
		return
	}
	// r63 self-gate: "When Aerith dies" — without it, a living Aerith
	// re-distributes its counter count onto every legendary on EVERY
	// creature death (creature_dies walks the whole battlefield). The
	// isLeavingPermEvent fallback delivers ctx["perm"]==perm on self-death.
	if p, _ := ctx["perm"].(*gameengine.Permanent); p != perm {
		return
	}
	x := 0
	if perm.Counters != nil {
		x = perm.Counters["+1/+1"]
	}
	if x <= 0 {
		emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
			"seat":     perm.Controller,
			"counters": 0,
		})
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p == perm || !p.IsCreature() || p.Card == nil {
			continue
		}
		if !cardHasType(p.Card, "legendary") {
			continue
		}
		p.AddCounter("+1/+1", x)
		count++
	}
	if count > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":            perm.Controller,
		"counters_each":   x,
		"legendary_count": count,
	})
}
