package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLordXanderTheCollector wires Lord Xander, the Collector.
//
// Oracle text:
//
//   When Lord Xander enters, target opponent discards half the cards in their hand, rounded down.
//   Whenever Lord Xander attacks, defending player mills half their library, rounded down.
//   When Lord Xander dies, target opponent sacrifices half the nonland permanents they control of their choice, rounded down.
//
// Auto-generated ETB handler.
func registerLordXanderTheCollector(r *Registry) {
	r.OnETB("Lord Xander, the Collector", lordXanderTheCollectorETB)
}

func lordXanderTheCollectorETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lord_xander_the_collector_etb"
	if gs == nil || perm == nil {
		return
	}
	seat := perm.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(), "auto-gen: ETB effect not parsed from oracle text")
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat": seat,
	})
}
