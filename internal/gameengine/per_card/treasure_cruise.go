package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerTreasureCruise wires Treasure Cruise.
//
// Oracle text (verified via hexdek.dev/api/oracle/card/Treasure%20Cruise):
//
//	Delve (Each card you exile from your graveyard while casting this
//	spell pays for {1}.)
//	Draw three cards.
//
// {7}{U} Sorcery. The Delve alt-cost is the value engine — typical
// cast is "exile 6-7 graveyard cards, pay {U}, draw 3". The delve
// cost reduction is handled by the engine's AST keyword pipeline
// (Delve is one of the recognized payment keywords). This handler
// implements only the resolve effect: draw three.
func registerTreasureCruise(r *Registry) {
	r.OnResolve("Treasure Cruise", treasureCruiseResolve)
}

func treasureCruiseResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "treasure_cruise"
	if gs == nil || item == nil {
		return
	}
	seat := item.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	drew := 0
	for i := 0; i < 3; i++ {
		if drawOne(gs, seat, "Treasure Cruise") != nil {
			drew++
		}
	}
	emit(gs, slug, "Treasure Cruise", map[string]interface{}{
		"seat":  seat,
		"drew":  drew,
	})
}
