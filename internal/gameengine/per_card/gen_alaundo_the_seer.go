package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerAlaundoTheSeer wires Alaundo the Seer.
//
// Oracle text:
//
//	{T}: Draw a card, then exile a card from your hand and put a number
//	of time counters on it equal to its mana value. It gains "When the
//	last time counter is removed from this card, if it's exiled, you
//	may cast it without paying its mana cost. If you cast a creature
//	spell this way, it gains haste until end of turn." Then remove a
//	time counter from each other card you own in exile.
//
// Implementation:
//
// True per-card time-counter tracking on cards in exile would require a
// new persistent side-channel (cards in exile have no Flags map). To
// model the deck's strategic value without that infrastructure, we
// approximate the suspend-then-cheat engine:
//
//   - Every activation draws a card.
//   - Every activation exiles the highest-CMC permanent card from hand
//     (matching the player's natural play of suspending a fattie).
//   - We tally activations on the seat. On the third activation, we
//     "release" the most-recently exiled-by-Alaundo permanent card —
//     ETB it onto the controller's battlefield, simulating the
//     time-counter timeout for a CMC-3 suspend (the deck's typical
//     average; bigger fatties pop after more activations naturally
//     since they exile and the third-tick chooses the most recent).
//   - Counter resets after a release.
//
// This isn't a faithful CR §702.62 suspend model but it captures the
// "tap once per turn → eventually cheat a permanent into play" loop
// the deck is built around. Without this the handler was a 4-CMC
// "draw a card" engine, which underweights the commander in MCTS
// rollouts and tournament winrate.
func registerAlaundoTheSeer(r *Registry) {
	r.OnActivated("Alaundo the Seer", alaundoTheSeerActivate)
}

const alaundoActivationsKey = "alaundo_activations"
const alaundoSuspendedKey = "alaundo_suspended_count"
const alaundoReleaseInterval = 3

func alaundoTheSeerActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	const slug = "alaundo_the_seer_activate"
	if gs == nil || src == nil {
		return
	}
	seatIdx := src.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil || seat.Lost {
		return
	}

	drew := drawOne(gs, seatIdx, src.Card.DisplayName())

	// Pick highest-CMC permanent from hand to "suspend".
	var suspendIdx int = -1
	suspendCMC := -1
	for i, c := range seat.Hand {
		if c == nil {
			continue
		}
		if !isPermanentCard(c) {
			continue
		}
		cmc := gameengine.ManaCostOf(c)
		if cmc > suspendCMC {
			suspendCMC = cmc
			suspendIdx = i
		}
	}

	suspended := ""
	if suspendIdx >= 0 {
		card := seat.Hand[suspendIdx]
		// MoveCard handles the hand removal. The manual splice before
		// this call was redundant (Wave 2 multi-step migration — same
		// shape as Cluster 1 in PR #924).
		gameengine.MoveCard(gs, card, seatIdx, "hand", "exile", "alaundo_suspend")
		suspended = card.DisplayName()
		if seat.Flags == nil {
			seat.Flags = map[string]int{}
		}
		seat.Flags[alaundoSuspendedKey]++
	}

	// Tally activation; every Nth activation, "release" the most recent
	// alaundo-suspended permanent from exile to the battlefield.
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	seat.Flags[alaundoActivationsKey]++
	released := ""
	if seat.Flags[alaundoSuspendedKey] > 0 && seat.Flags[alaundoActivationsKey]%alaundoReleaseInterval == 0 {
		// Walk exile from the back (most recently suspended) and find a
		// permanent card to release.
		for i := len(seat.Exile) - 1; i >= 0; i-- {
			ex := seat.Exile[i]
			if ex == nil || !isPermanentCard(ex) {
				continue
			}
			// MoveCard handles the exile removal + battlefield placement;
			// the manual splice was redundant (Wave 2 multi-step migration).
			// §108.3 (r63): `ex` is the seat's own suspended card —
			// Card.Owner already correct; mutation removed.
			gameengine.MoveCard(gs, ex, seatIdx, "exile", "battlefield", "alaundo_release")
			released = ex.DisplayName()
			seat.Flags[alaundoSuspendedKey]--
			break
		}
	}

	drawnName := ""
	if drew != nil {
		drawnName = drew.DisplayName()
	}
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":            seatIdx,
		"drew":            drawnName,
		"suspended":       suspended,
		"suspend_cmc":     suspendCMC,
		"released":        released,
		"activations":     seat.Flags[alaundoActivationsKey],
		"suspended_count": seat.Flags[alaundoSuspendedKey],
	})
	if released == "" && suspended == "" {
		emitPartial(gs, slug, src.Card.DisplayName(),
			"per_card_time_counter_tracking_unimplemented_using_activation_count_proxy")
	}
}
