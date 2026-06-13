package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSeizanPerverterOfTruth wires Seizan, Perverter of Truth.
//
// Oracle text (Scryfall, verified 2026-06-13):
//
//	At the beginning of each player's upkeep, that player loses 2 life and
//	draws two cards.
//
// Why a per-card handler: the symmetric upkeep ability parses to an inert
// `trigger_fragment_upkeep` node with no effect — generic dispatch neither
// drains life nor draws. Seizan is a popular group-draw / aristocrats payoff;
// inert it does nothing.
//
// Implementation (CR §603 triggered ability, symmetric):
//   - Listen on "upkeep". The ctx carries "active_seat" = the player whose
//     upkeep it is; that player is the one affected (NOT gated to controller).
//   - That player loses 2 life, then draws two cards (DrawN — full §614.11
//     fidelity).
//
// Self-registers via init() + AddResetHook (no shared registry edits).
func init() {
	registerSeizanPerverterOfTruth(Global())
	AddResetHook(registerSeizanPerverterOfTruth)
}

func registerSeizanPerverterOfTruth(r *Registry) {
	r.OnTrigger("Seizan, Perverter of Truth", "upkeep", seizanUpkeep)
}

func seizanUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "seizan_perverter_upkeep_drain_draw"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok || activeSeat < 0 || activeSeat >= len(gs.Seats) {
		return
	}
	if gs.Seats[activeSeat] == nil || gs.Seats[activeSeat].Lost {
		return
	}

	gameengine.LoseLife(gs, activeSeat, 2, perm.Card.DisplayName())
	drawn := gameengine.DrawN(gs, activeSeat, 2, perm)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":      activeSeat,
		"life_lost": 2,
		"drawn":     drawn,
	})
}
