package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// Beacon of Immortality — {4}{W} Instant.
//
//	Double target player's life total.
//	Shuffle Beacon of Immortality into its owner's library.
//
// The whole effect parsed to a `custom` slug with no handler, so the
// spell resolved to a no-op and nobody's life doubled. This handler
// doubles the targeted player's life (CR 119.4 — a player gains life
// equal to their current total).
//
// Scope: the "shuffle Beacon into its owner's library instead of the
// graveyard" clause is a self-zone-replacement that the resolve pipeline
// routes after the per-card hook returns; it is NOT modeled here, so the
// card goes to the graveyard like a normal instant. Logged in the
// coverage report.
func init() {
	registerBeaconOfImmortality(Global())
	AddResetHook(registerBeaconOfImmortality)
}

func registerBeaconOfImmortality(r *Registry) {
	r.OnResolve("Beacon of Immortality", beaconOfImmortalityResolve)
}

func beaconOfImmortalityResolve(gs *gameengine.GameState, item *gameengine.StackItem) {
	const slug = "beacon_of_immortality"
	if gs == nil || item == nil {
		return
	}
	seat := -1
	for _, t := range item.Targets {
		if t.Kind == gameengine.TargetKindSeat {
			seat = t.Seat
			break
		}
	}
	if seat < 0 {
		seat = item.Controller // robustness: default to caster if no seat target resolved
	}
	if seat < 0 || seat >= len(gs.Seats) || gs.Seats[seat] == nil {
		emitFail(gs, slug, "Beacon of Immortality", "bad_target", nil)
		return
	}
	cur := gs.Seats[seat].Life
	if cur > 0 {
		gameengine.GainLife(gs, seat, cur, "Beacon of Immortality")
	}
	emit(gs, slug, "Beacon of Immortality", map[string]interface{}{
		"seat":        seat,
		"life_before": cur,
		"life_after":  gs.Seats[seat].Life,
	})
}
