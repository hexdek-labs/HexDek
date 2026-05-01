package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLorehold wires Lorehold, the Historian.
//
// Oracle text:
//
//	Flying, haste
//	Each instant and sorcery card in your hand has miracle {2}.
//	(You may cast a card for its miracle cost when you draw it if it's
//	the first card you drew this turn.)
//	At the beginning of each opponent's upkeep, you may discard a card.
//	If you do, draw a card.
//
// Implementation:
//   - ETB: emitPartial for the miracle alt-cost grant. Granting miracle
//     to instants/sorceries in hand requires hooking the draw step's
//     "first card drawn this turn" detection and re-pricing those cards
//     at {2}; non-trivial.
//   - upkeep_controller (filtered to opponents): may discard a card; if
//     so, draw one. AI heuristic: always loot when hand non-empty.
func registerLorehold(r *Registry) {
	r.OnETB("Lorehold, the Historian", loreholdETB)
	r.OnTrigger("Lorehold, the Historian", "upkeep_controller", loreholdOpponentUpkeep)
}

func loreholdETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "lorehold_historian_miracle_grant"
	if gs == nil || perm == nil {
		return
	}
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"miracle_2_alt_cost_for_instant_sorcery_in_hand_unimplemented")
}

func loreholdOpponentUpkeep(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "lorehold_historian_loot"
	if gs == nil || perm == nil || ctx == nil {
		return
	}
	activeSeat, ok := ctx["active_seat"].(int)
	if !ok {
		return
	}
	if activeSeat == perm.Controller {
		return // only opponents' upkeeps
	}
	seat := perm.Controller
	s := gs.Seats[seat]
	if s == nil || s.Lost {
		return
	}
	if len(s.Hand) == 0 {
		return
	}
	discarded := gameengine.DiscardN(gs, seat, 1, "")
	if discarded <= 0 {
		return
	}
	drawOne(gs, seat, perm.Card.DisplayName())
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":        seat,
		"active_seat": activeSeat,
		"discarded":   discarded,
		"drew":        1,
	})
}
