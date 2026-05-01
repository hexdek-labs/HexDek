package per_card

import (
	"github.com/hexdek/hexdek/internal/gameengine"
)

// registerLordWindgrace wires Lord Windgrace.
//
// Oracle text (Scryfall, verified 2026-05-01):
//
//	Legendary Planeswalker — Windgrace. Loyalty 5.
//	+2: Discard a card. If a land card is discarded this way, return it
//	    to the battlefield. Otherwise, draw a card.
//	−3: Return up to two target land cards from your graveyard to the
//	    battlefield.
//	−11: Destroy each nonland permanent. Search your library for any
//	     number of land cards, put them onto the battlefield, then
//	     shuffle.
//	Lord Windgrace can be your commander.
//
// The engine has no native loyalty-cost framework for planeswalker
// activations; the handler manages loyalty directly via
// perm.Counters["loyalty"] += delta (mirrors Tevesh Szat / Dihada).
//
// Activation indexing follows oracle order: 0 = +2, 1 = −3, 2 = −11.
func registerLordWindgrace(r *Registry) {
	r.OnETB("Lord Windgrace", lordWindgraceETB)
	r.OnActivated("Lord Windgrace", lordWindgraceActivate)
}

func lordWindgraceETB(gs *gameengine.GameState, perm *gameengine.Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Counters == nil {
		perm.Counters = map[string]int{}
	}
	perm.Counters["loyalty"] = 5
}

func lordWindgraceActivate(gs *gameengine.GameState, src *gameengine.Permanent, abilityIdx int, ctx map[string]interface{}) {
	if gs == nil || src == nil || src.Card == nil {
		return
	}
	switch abilityIdx {
	case 0:
		lordWindgracePlusTwo(gs, src)
	case 1:
		lordWindgraceMinusThree(gs, src)
	case 2:
		lordWindgraceMinusEleven(gs, src)
	}
}

func lordWindgracePlusTwo(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "lord_windgrace_plus_two_discard"
	src.AddCounter("loyalty", 2)

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	if len(s.Hand) == 0 {
		// Discarding a card with an empty hand still resolves but discards
		// nothing; printed text doesn't make discard mandatory beyond "if
		// you do" semantics, but oracle says "Discard a card." If no card
		// to discard, the rest of the effect doesn't trigger.
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":     seat,
			"loyalty":  src.Counters["loyalty"],
			"discarded": "",
			"land":     false,
			"drew":     false,
			"reason":   "empty_hand",
		})
		return
	}

	// Choose a card to discard. Prefer lands (so we get the bring-back),
	// otherwise the lowest-CMC non-land in hand (cheapest fodder).
	pick := lordWindgracePickDiscard(s.Hand)
	pickName := pick.DisplayName()
	wasLand := cardHasType(pick, "land")

	gameengine.DiscardCard(gs, pick, seat)

	drew := false
	returned := false
	if wasLand {
		// Return the discarded land from the graveyard to the battlefield.
		// The discard moved it to graveyard via the standard MoveCard
		// pipeline, so it should be there now (unless a §614 replacement
		// rerouted it — e.g. Necropotence's "discard exiles" replacement,
		// which DiscardCard handles by routing to exile; in that case the
		// land is in exile and this lookup misses, which matches rules:
		// "If a land card is discarded this way" — but the card was sent
		// to exile, not graveyard, so it isn't returnable from there).
		if cardInZone(s.Graveyard, pick) {
			gameengine.MoveCard(gs, pick, seat, "graveyard", "battlefield", "lord_windgrace_plus_two")
			enterBattlefieldWithETB(gs, seat, pick, false)
			returned = true
		}
	} else {
		if drawOne(gs, seat, src.Card.DisplayName()) != nil {
			drew = true
		}
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"loyalty":   src.Counters["loyalty"],
		"discarded": pickName,
		"land":      wasLand,
		"returned":  returned,
		"drew":      drew,
	})
}

func lordWindgraceMinusThree(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "lord_windgrace_minus_three_recur_lands"
	src.AddCounter("loyalty", -3)

	seat := src.Controller
	if seat < 0 || seat >= len(gs.Seats) {
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	// Pick up to two land cards from controller's graveyard. Prefer
	// non-basic lands (utility / dual lands are usually higher impact than
	// basics), tiebreak by traversal order.
	picks := lordWindgracePickGraveyardLands(s.Graveyard, 2)
	if len(picks) == 0 {
		emitFail(gs, slug, src.Card.DisplayName(), "no_lands_in_graveyard", map[string]interface{}{
			"seat":    seat,
			"loyalty": src.Counters["loyalty"],
		})
		return
	}

	returnedNames := make([]string, 0, len(picks))
	for _, c := range picks {
		gameengine.MoveCard(gs, c, seat, "graveyard", "battlefield", "lord_windgrace_minus_three")
		enterBattlefieldWithETB(gs, seat, c, false)
		returnedNames = append(returnedNames, c.DisplayName())
	}

	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":     seat,
		"loyalty":  src.Counters["loyalty"],
		"returned": returnedNames,
		"count":    len(returnedNames),
	})
}

func lordWindgraceMinusEleven(gs *gameengine.GameState, src *gameengine.Permanent) {
	const slug = "lord_windgrace_minus_eleven_apocalypse"
	src.AddCounter("loyalty", -11)

	seat := src.Controller

	// 1. Destroy each nonland permanent across all seats.
	var victims []*gameengine.Permanent
	for _, sp := range gs.Seats {
		if sp == nil {
			continue
		}
		for _, p := range sp.Battlefield {
			if p == nil || p.Card == nil {
				continue
			}
			if p.IsLand() {
				continue
			}
			victims = append(victims, p)
		}
	}
	destroyed := 0
	for _, p := range victims {
		if gameengine.DestroyPermanent(gs, p, src) {
			destroyed++
		}
	}

	// 2. Search library for any number of land cards, put them onto the
	//    battlefield, then shuffle. MVP: take every land in library.
	if seat < 0 || seat >= len(gs.Seats) {
		emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
			"seat":      seat,
			"destroyed": destroyed,
			"fetched":   0,
		})
		return
	}
	s := gs.Seats[seat]
	if s == nil {
		return
	}

	var fetched []*gameengine.Card
	keep := s.Library[:0]
	for _, c := range s.Library {
		if c != nil && cardHasType(c, "land") {
			fetched = append(fetched, c)
			continue
		}
		keep = append(keep, c)
	}
	s.Library = keep

	for _, c := range fetched {
		// Card has already been removed from the library above; skip
		// MoveCard's removal pass and create the permanent directly.
		// Fire zone-change triggers manually so etb dispatch &
		// observers see the entry.
		enterBattlefieldWithETB(gs, seat, c, false)
	}
	shuffleLibraryPerCard(gs, seat)

	gs.LogEvent(gameengine.Event{
		Kind:   "search_library",
		Seat:   seat,
		Source: src.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason":      "lord_windgrace_minus_eleven",
			"destination": "battlefield",
			"found_count": len(fetched),
		},
	})
	emit(gs, slug, src.Card.DisplayName(), map[string]interface{}{
		"seat":      seat,
		"loyalty":   src.Counters["loyalty"],
		"destroyed": destroyed,
		"fetched":   len(fetched),
	})
	_ = gs.CheckEnd()
}

// lordWindgracePickDiscard returns the card to discard for the +2 ability.
// Strict order:
//  1. First land in hand (always optimal — gets returned to battlefield).
//  2. Lowest-CMC non-land card otherwise (cheapest fodder; you draw a
//     replacement so this is a slight upgrade).
func lordWindgracePickDiscard(hand []*gameengine.Card) *gameengine.Card {
	for _, c := range hand {
		if c != nil && cardHasType(c, "land") {
			return c
		}
	}
	var best *gameengine.Card
	bestCMC := 1<<31 - 1
	for _, c := range hand {
		if c == nil {
			continue
		}
		cmc := cardCMC(c)
		if cmc < bestCMC {
			bestCMC = cmc
			best = c
		}
	}
	if best == nil && len(hand) > 0 {
		return hand[0]
	}
	return best
}

// lordWindgracePickGraveyardLands returns up to n land cards from the
// graveyard, preferring non-basic lands (higher value targets like
// duals/fetches). Stops at n picks.
func lordWindgracePickGraveyardLands(graveyard []*gameengine.Card, n int) []*gameengine.Card {
	if n <= 0 {
		return nil
	}
	out := make([]*gameengine.Card, 0, n)
	// Pass 1: non-basic lands.
	for _, c := range graveyard {
		if c == nil || !cardHasType(c, "land") {
			continue
		}
		if cardHasType(c, "basic") {
			continue
		}
		out = append(out, c)
		if len(out) >= n {
			return out
		}
	}
	// Pass 2: any remaining land (basics + anything missed).
	if len(out) < n {
		for _, c := range graveyard {
			if c == nil || !cardHasType(c, "land") {
				continue
			}
			if containsCardPtr(out, c) {
				continue
			}
			out = append(out, c)
			if len(out) >= n {
				return out
			}
		}
	}
	return out
}

func cardInZone(zone []*gameengine.Card, want *gameengine.Card) bool {
	for _, c := range zone {
		if c == want {
			return true
		}
	}
	return false
}

func containsCardPtr(slice []*gameengine.Card, want *gameengine.Card) bool {
	for _, c := range slice {
		if c == want {
			return true
		}
	}
	return false
}
