package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSidarKondo wires Sidar Kondo of Jamuraa.
//
// Oracle text:
//
//	Flanking
//	Creatures with power 2 or less can't be blocked by creatures with
//	power 3 or greater.
//	Partner
//
// Flanking and Partner are AST-parsed keywords. The blocking restriction
// is a global static affecting ALL creatures (regardless of controller)
// while any Sidar Kondo is on a battlefield. We model it by stamping
// gs.Flags["sidar_kondo_active"] on ETB; combat.go's canBlockGS reads
// the flag (with a battlefield re-check) and rejects illegal blocks.
func registerSidarKondo(r *Registry) {
	r.OnETB("Sidar Kondo of Jamuraa", sidarKondoETB)
}

func sidarKondoETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["sidar_kondo_active"] = 1
	emit(gs, "sidar_kondo_etb", perm.Card.DisplayName(), map[string]interface{}{
		"seat":   perm.Controller,
		"effect": "small_creatures_evade_large_blockers",
	})
}
