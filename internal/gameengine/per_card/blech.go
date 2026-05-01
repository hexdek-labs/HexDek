package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerBlech wires Blech, Loafing Pest.
//
// Oracle text:
//
//	Whenever you gain life, put a +1/+1 counter on each Pest, Bat,
//	Insect, Snake, and Spider you control.
//
// Triggers on engine "life_gained" event when the gaining seat is
// Blech's controller. Walks the controller's battlefield and adds
// +1/+1 counters to every creature whose type line includes any of
// the listed subtypes.
func registerBlech(r *Registry) {
	r.OnTrigger("Blech, Loafing Pest", "life_gained", blechLifeGained)
}

func blechLifeGained(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "blech_loafing_pest_anthem"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	gainSeat, ok := ctx["seat"].(int)
	if !ok {
		return
	}
	if gainSeat != perm.Controller {
		return
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	subtypes := []string{"pest", "bat", "insect", "snake", "spider"}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		matched := false
		for _, st := range subtypes {
			if cardHasType(p.Card, st) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		p.AddCounter("+1/+1", 1)
		count++
	}
	if count > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":             perm.Controller,
		"creatures_buffed": count,
	})
}
