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
	emitPartial(gs, slug, perm.Card.DisplayName(),
		"search_replacement_callsites_must_consult_OppositionAgentControlsSearch")
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
// The exiled card is tagged on the controller's seat in exile; a
// future "play-from-exile" primitive will consult that tag to know
// Agent's controller can cast it.
func ExileSearchResult(gs *gameengine.GameState, controllerSeat int, card *gameengine.Card) {
	const slug = "opposition_agent_exile_search_result"
	if gs == nil || card == nil || controllerSeat < 0 || controllerSeat >= len(gs.Seats) {
		return
	}
	// Move to Agent-controller's exile zone (NOT the searcher's).
	// Card's owner is used for zone routing; the controller's exile holds it.
	gameengine.MoveCard(gs, card, controllerSeat, "library", "exile", "opposition-agent-exile")
	gs.LogEvent(gameengine.Event{
		Kind:   "opposition_agent_exile",
		Seat:   controllerSeat,
		Source: "Opposition Agent",
		Details: map[string]interface{}{
			"exiled_card":     card.DisplayName(),
			"controller_seat": controllerSeat,
			"rule":            "701.19",
		},
	})
	emit(gs, slug, "Opposition Agent", map[string]interface{}{
		"exiled_card":     card.DisplayName(),
		"controller_seat": controllerSeat,
	})
}
