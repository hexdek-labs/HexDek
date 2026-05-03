package per_card

import (
	"fmt"

	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerSefrisOfTheHiddenWays wires Sefris of the Hidden Ways.
//
// Oracle text (Adventures in the Forgotten Realms Commander, verified Scryfall 2026-05-02):
//
//	Whenever one or more creature cards are put into your graveyard from
//	  anywhere, venture into the dungeon. This ability triggers only once
//	  each turn.
//	Create Undead — Whenever you complete a dungeon, return target creature
//	  card from your graveyard to the battlefield.
//
// Implementation:
//
//   - OnTrigger("creature_dies"): fires when a creature enters a graveyard
//     from the battlefield (CR §700.4). Gate on controller_seat matching
//     Sefris's controller. Once-per-turn gate: perm.Flags keyed on turn
//     number to allow only one venture per turn. Call VentureIntoDungeon;
//     if the venture completes the dungeon (room == 4 or
//     seat.Flags["dungeon_completed"] > 0), fire the Create Undead
//     reanimation inline.
//
//   - OnTrigger("card_discarded"): fires when a card is discarded to the
//     graveyard from hand (covers the "from anywhere" clause for hand →
//     graveyard). Gate on discarder_seat and creature type. Same
//     once-per-turn lock as above.
//
//   - Mill (library → graveyard) gap: the engine does not fire a
//     FireCardTrigger for individual card mill events; it only uses
//     MoveCard + FireZoneChangeTriggers, which has no creature-card-to-
//     graveyard-from-library hook in the per-card trigger system. We
//     emit a partial to track the gap. In practice, mill-from-library
//     is a minor coverage gap — the most common paths (creature dies,
//     discard) are fully handled.
//
//   - Create Undead reanimation: after each successful venture that
//     completes the dungeon, return the highest-CMC creature card from
//     Sefris's controller's graveyard to the battlefield via MoveCard +
//     enterBattlefieldWithETB. Ties broken by graveyard order (latest
//     entry preferred — last in slice).
func registerSefrisOfTheHiddenWays(r *Registry) {
	r.OnTrigger("Sefris of the Hidden Ways", "creature_dies", sefrisDies)
	r.OnTrigger("Sefris of the Hidden Ways", "card_discarded", sefrisDiscard)
}

// sefrisDies handles "creature card put into graveyard from the battlefield."
func sefrisDies(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sefris_of_the_hidden_ways_creature_dies"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: the creature's controller must be Sefris's controller.
	controllerSeat, _ := ctx["controller_seat"].(int)
	if controllerSeat != perm.Controller {
		return
	}

	sefrisVenture(gs, perm, slug, "creature_dies", ctx)
}

// sefrisDiscard handles "creature card put into graveyard from hand."
func sefrisDiscard(gs *gameengine.GameState, perm *gameengine.Permanent, ctx map[string]interface{}) {
	const slug = "sefris_of_the_hidden_ways_card_discarded"
	if gs == nil || perm == nil || ctx == nil {
		return
	}

	// Gate: the discarder must be Sefris's controller.
	discarderSeat, _ := ctx["discarder_seat"].(int)
	if discarderSeat != perm.Controller {
		return
	}

	// Gate: the discarded card must be a creature card.
	card, _ := ctx["card"].(*gameengine.Card)
	if card == nil || !cardHasType(card, "creature") {
		return
	}

	// Gate: card must actually reach the graveyard (not exiled by
	// Necropotence or similar replacement effects).
	if exiled, _ := ctx["exiled"].(bool); exiled {
		return
	}

	sefrisVenture(gs, perm, slug, "card_discarded", ctx)
}

// sefrisVenture is the shared once-per-turn venture core. It enforces the
// "triggers only once each turn" clause, calls VentureIntoDungeon, and
// triggers Create Undead if a dungeon is completed.
func sefrisVenture(gs *gameengine.GameState, perm *gameengine.Permanent, slug, source string, ctx map[string]interface{}) {
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}

	// Once-per-turn gate (CR §603.2b "triggers only once each turn").
	// Key encodes the turn number so the lock resets automatically each turn.
	dedupeKey := fmt.Sprintf("sefris_last_venture_turn_%d", gs.Turn)
	if perm.Flags[dedupeKey] == 1 {
		// Already ventured this turn; skip.
		return
	}
	perm.Flags[dedupeKey] = 1

	seat := gs.Seats[perm.Controller]
	if seat == nil || seat.Lost {
		return
	}

	// Emit the partial for the mill coverage gap once per game (keyed to turn
	// 0 so it surfaces on the first venture regardless of which trigger fired).
	if perm.Flags["sefris_mill_gap_emitted"] == 0 {
		perm.Flags["sefris_mill_gap_emitted"] = 1
		emitPartial(gs, slug, perm.Card.DisplayName(),
			"creature_card_mill_library_to_graveyard_not_covered_no_per_card_trigger_in_engine")
	}

	// Venture into the dungeon.
	room := gameengine.VentureIntoDungeon(gs, perm.Controller)

	emit(gs, slug, perm.Card.DisplayName(), map[string]interface{}{
		"controller": perm.Controller,
		"room":       room,
		"source":     source,
		"turn":       gs.Turn,
	})

	// Create Undead — if the venture completed a dungeon (room 4 is the
	// final room in the engine's simplified 4-room model; the flag is also
	// set by VentureIntoDungeon before SBAs clear it).
	if seat.Flags == nil {
		seat.Flags = map[string]int{}
	}
	dungeonDone := (room == 4) || (seat.Flags["dungeon_completed"] > 0)
	if !dungeonDone {
		return
	}

	sefrisCreateUndead(gs, perm, slug)
}

// sefrisCreateUndead returns the best creature card from Sefris's
// controller's graveyard to the battlefield.
//
// "Best" heuristic: highest CMC (returning the most expensive creature
// maximises tempo). Ties are broken by graveyard order — the card
// most recently added to the graveyard (last index) is preferred, matching
// typical player intuition of reanimating what was just discarded.
func sefrisCreateUndead(gs *gameengine.GameState, perm *gameengine.Permanent, slug string) {
	const reanimSlug = "sefris_of_the_hidden_ways_create_undead"
	seat := gs.Seats[perm.Controller]
	if seat == nil {
		return
	}

	var best *gameengine.Card
	bestCMC := -1
	bestIdx := -1
	for i, c := range seat.Graveyard {
		if c == nil || !cardHasType(c, "creature") {
			continue
		}
		cmc := cardCMC(c)
		if cmc > bestCMC || (cmc == bestCMC && i > bestIdx) {
			bestCMC = cmc
			bestIdx = i
			best = c
		}
	}

	if best == nil {
		emitFail(gs, reanimSlug, perm.Card.DisplayName(), "no_creature_in_graveyard", map[string]interface{}{
			"controller": perm.Controller,
		})
		return
	}

	// Move from graveyard to battlefield, then fire ETB triggers.
	gameengine.MoveCard(gs, best, perm.Controller, "graveyard", "battlefield", "sefris_create_undead")
	enterBattlefieldWithETB(gs, perm.Controller, best, false)

	emit(gs, reanimSlug, perm.Card.DisplayName(), map[string]interface{}{
		"controller": perm.Controller,
		"returned":   best.DisplayName(),
		"cmc":        bestCMC,
	})

	_ = gs.CheckEnd()
}
