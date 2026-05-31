package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerElrondMasterOfHealing wires Elrond, Master of Healing.
//
// Oracle text:
//
//	Whenever you scry, put a +1/+1 counter on each of up to X target
//	creatures, where X is the number of cards looked at while scrying
//	this way.
//	Whenever a creature you control with a +1/+1 counter on it becomes
//	the target of a spell or ability an opponent controls, you may draw
//	a card.
//
// Both triggers depend on hooks the engine doesn't yet expose; stub.
func registerElrondMasterOfHealing(r *Registry) {
	r.OnTrigger("Elrond, Master of Healing", "scry", elrondScry)
}

func elrondScry(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "elrond_scry_counters"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	scryerSeat, _ := ctx["seat"].(int)
	if scryerSeat != perm.Controller {
		return
	}
	// Engine fires scry with "amount" (scry.go:75). Legacy "count" kept
	// as a fallback for callers that thread it explicitly.
	x, ok := ctx["amount"].(int)
	if !ok {
		x, _ = ctx["count"].(int)
	}
	if x <= 0 {
		x = 1
	}
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}
	count := 0
	for _, p := range seat.Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		p.AddCounter("+1/+1", 1)
		count++
		if count >= x {
			break
		}
	}
	if count > 0 {
		gs.InvalidateCharacteristicsCache()
	}
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"buffed":   count,
		"x":        x,
	})
}
