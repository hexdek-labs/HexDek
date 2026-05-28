package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerOppositionAgent wires up Opposition Agent.
//
// Oracle text:
//
//	Flash
//	You control your opponents' searches.
//	Whenever an opponent searches a library, you exile each card they
//	find. You may play those cards for as long as they remain exiled,
//	and you may spend mana as though it were mana of any color to
//	cast those spells.
//
// Tier-one cEDH stax creature. Drops in response to a Demonic Tutor /
// Vampiric Tutor / fetchland crack — the searching player now hands
// the tutored card to Opposition Agent's controller, and the Agent's
// player gets to cast it. Single-handedly defeats tutor-heavy combo
// decks.
//
// Batch #3 scope:
//   - OnETB: stamp gs.Flags["opposition_agent_seat_N"] so any engine-
//     side search-library primitive can consult it.
//   - Provide OppositionAgentControlsSearch(gs, searchingSeat) helper
//     that returns the CONTROLLING seat (i.e. the seat that gets to
//     exile & play the found card). Returns -1 when no Agent is
//     active.
//   - ExileSearchResult(gs, controllerSeat, card) — the actual
//     exile-and-grant-play primitive for callers that perform a
//     search on behalf of an opponent.
//
// The "control the search" clause means Agent's controller chooses
// what to find — they can choose NOTHING, which is a potent stax
// effect (negates fetchlands, Demonic Tutor, Birthing Pod activations,
// etc.). Our helper accommodates this via the exile-no-matter-what
// path: ExileSearchResult always moves the card to exile.
//
// Full §701.19 replacement wiring is engine-side (the search path
// would need replacement effect registration similar to Commander
// loss). For now we provide the hooks + flag so search call sites
// can consult them when the search primitive lands.
func registerOppositionAgent(r *Registry) {
	r.OnETB("Opposition Agent", oppositionAgentETB)
}

func oppositionAgentETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	const slug = "opposition_agent_static"
	if gs == nil || perm == nil {
		return
	}
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["opposition_agent_seat_"+intToStr(perm.Controller)] = perm.Timestamp
	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"seat":     perm.Controller,
		"controls": "opponent_library_searches",
		"rule":     "701.19",
	})
}

// OppositionAgentControlsSearch returns the seat index of the player
// who gets to control a search when `searchingSeat` is attempting to
// search a library (§701.19). Returns -1 when no Agent is active
// among opposing seats. The first Agent found (lowest seat idx) wins
// — this mirrors "apply replacement effects in APNAP order" for
// search-control replacements.
func OppositionAgentControlsSearch(gs *gameengine.GameState, searchingSeat int) int {
	if gs == nil || gs.Flags == nil {
		return -1
	}
	for i := range gs.Seats {
		if i == searchingSeat {
			continue
		}
		if gs.Flags["opposition_agent_seat_"+intToStr(i)] > 0 {
			return i
		}
	}
	return -1
}

// ExileSearchResult is the engine-side primitive for "opponent
// searches for a card; it's exiled by Opposition Agent's controller
// instead of going to the searcher's hand". Callers that perform a
// search on behalf of an opponent MUST check
// OppositionAgentControlsSearch first; if it returns a non-(-1) seat,
// they pass the found card here.
//
// The exiled card lands in its OWNER's exile zone per CR §400.7 /
// §406.1 and Wizards' 2020-09-25 Scryfall ruling on Opposition Agent:
// "When you search an opponent's library, you exile any of the cards
// that were found. The exiled cards are exiled into THAT PLAYER'S
// exile zone." Agent's controller chooses what to exile, but the
// destination zone is the searcher's (owner's) exile. Pre-r60 this
// function and the §400.7 audit (PR #693) caught the same Etali-shape
// cross-seat exile bug surfaced in PR #685. The Agent's right to
// CAST the exiled card is preserved via the ZoneCastGrant registered
// by future work; cast permission is independent of zone-of-residence.
func ExileSearchResult(gs *gameengine.GameState, controllerSeat int, card *gameengine.Card) {
	const slug = "opposition_agent_exile_search_result"
	if gs == nil || card == nil || controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return
	}
	// Route to OWNER's exile per CR §400.7. MoveCard normalises the
	// destination through moveToZone, which (since PR #693) now also
	// enforces this redirect defensively at the API boundary.
	ownerSeat := card.Owner
	if ownerSeat < 0 || ownerSeat >= len(gs.Seats) {
		ownerSeat = controllerSeat
	}
	gameengine.MoveCard(gs, card, ownerSeat, "library", "exile", "opposition-agent-exile")
	gs.LogEvent(gameengine.Event{
		Kind:   "opposition_agent_exile",
		Seat:   controllerSeat,
		Source: "Opposition Agent",
		Details: map[string]interface{}{
			"exiled_card":     card.DisplayName(),
			"controller_seat": controllerSeat,
			"owner_seat":      ownerSeat,
			"rule":            "701.19",
		},
	})
	emit(gs, slug, "Opposition Agent", map[string]interface{}{
		"exiled_card":     card.DisplayName(),
		"controller_seat": controllerSeat,
		"owner_seat":      ownerSeat,
	})
}
