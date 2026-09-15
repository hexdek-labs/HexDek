package gameengine

import "strings"

// keywords_discover.go — CR §701.57 Discover N (Lost Caverns of Ixalan,
// 2023).
//
// "Discover N — Exile cards from the top of your library until you
//  exile a nonland card with mana value N or less. You may cast that
//  card without paying its mana cost. If you don't, put it into your
//  hand. Then put all other cards exiled this way on the bottom of
//  your library in a random order."
//
// Discover is closely related to Cascade (§702.84) but with three
// differences:
//
//   1. Stop condition is "CMC <= N", not "CMC < spellCMC".
//   2. The controller may put the hit into their HAND instead of
//      casting it for free. Cascade has no hand option.
//   3. N is an explicit numeric parameter, not derived from the
//      triggering spell's mana value.
//
// Architecture:
//
//   - HasDiscover(card)               keyword detection (defensive —
//                                     discover usually shows up as
//                                     reminder text rather than a clean
//                                     keyword, but newer printings DO
//                                     carry the keyword token).
//   - DiscoverCount(card)             extract N from the keyword args
//                                     when present, otherwise 0.
//   - NewDiscoverCastFromExilePermission(owner) builds the
//                                     ZoneCastPermission that lets the
//                                     owner cast the discovered card
//                                     from exile for {0} mana.
//   - ApplyDiscover(gs, seat, n)      default-choice entry point;
//                                     casts the hit for free per the
//                                     prompt's spec ("default cast-free
//                                     per typical play").
//   - ApplyDiscoverWithChoice(...)    explicit-choice variant —
//                                     controller picks
//                                     DiscoverChoiceCastFree or
//                                     DiscoverChoiceToHand.
//
// Cards exiled during discover travel exile briefly: the misses come
// back to the library bottom (in a randomized order) via the standard
// MoveCard path so §614 replacement effects and exile-leave triggers
// fire correctly. The hit, when cast for free, is cast FROM EXILE via
// a one-shot ZoneCastPermission whose ManaCost=0; the resulting
// StackItem carries CostMeta["discover_cast"]=true +
// ["alt_cost"]="discover" + CastZone=ZoneExile.

// DiscoverChoice enumerates the two CR §701.51 dispositions for the
// hit card.
type DiscoverChoice int

const (
	// DiscoverChoiceCastFree casts the hit from exile for {0}. Default
	// per the prompt's spec — most discover triggers are net-positive
	// when free-cast, and reservation of the hand-route to the caller
	// keeps the default behavior aligned with strong AI play.
	DiscoverChoiceCastFree DiscoverChoice = iota
	// DiscoverChoiceToHand puts the hit into the caster's hand without
	// casting it. Used when the hit is uncastable (color requirements,
	// timing) or when holding it is strictly better.
	DiscoverChoiceToHand
)

// HasDiscover reports whether the card carries the discover keyword.
func HasDiscover(card *Card) bool {
	return cardHasKeywordByName(card, "discover")
}

// DiscoverCount returns N from a "discover N" keyword.
func DiscoverCount(card *Card) int {
	return keywordArgCost(card, "discover")
}

// NewDiscoverCastFromExilePermission builds a one-shot
// ZoneCastPermission allowing `owner` to cast a card from exile for
// {0} mana, tagged as a discover free-cast. Cleared after use by the
// caller (ApplyDiscover removes the grant once the stack item is
// pushed; the permission is a marker for any cast-pipeline hook that
// might consult ZoneCastGrants between push and resolve).
//
// RequireController=`owner` locks the grant to the discoverer; an
// opponent cannot piggyback the free cast even if they somehow get
// priority on the exiled card.
//
// Duration is empty (permanent until consumed) since the cast happens
// inline as part of ApplyDiscover — there's no priority-pass window
// where a stale grant would matter.
func NewDiscoverCastFromExilePermission(owner int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "discover",
		ManaCost:          0,
		RequireController: owner,
		SourceName:        "discover",
		Duration:          "",
	}
}

// ApplyDiscover runs Discover N with the default cast-free
// disposition. Returns the discovered (hit) card, or nil if the
// procedure whiffed (no qualifying nonland found before the library
// emptied or the depth was exhausted).
//
// Equivalent to ApplyDiscoverWithChoice(gs, seatIdx, n,
// DiscoverChoiceCastFree).
func ApplyDiscover(gs *GameState, seatIdx, n int) *Card {
	return ApplyDiscoverWithChoice(gs, seatIdx, n, DiscoverChoiceCastFree)
}

// ApplyDiscoverWithChoice runs Discover N and applies the specified
// disposition to the hit card. CR §701.51.
//
// Procedure:
//  1. Log discover_trigger.
//  2. Exile cards from the top of the library one at a time. Lands
//     keep going; the first nonland with CMC <= n is the hit. The
//     loop also stops if the library empties (CR §701.57a).
//  3. If a hit was found:
//     - choice == DiscoverChoiceCastFree: register the
//     ZoneCastPermission, build a StackItem with
//     Effect=collectSpellEffect(card),
//     CostMeta["discover_cast"]=true, ["alt_cost"]="discover",
//     ["discover_n"]=n, CastZone=ZoneExile, push it, then
//     resolve it inline. The cast-cost is {0}; ManaPool is
//     untouched. Once the item is pushed we drop the grant so
//     subsequent cast checks don't accidentally consume it.
//     - choice == DiscoverChoiceToHand: MoveCard from exile to
//     hand via the standard zone-move path.
//     Hit cards never get put back on the bottom of the library by
//     the discover machinery — once cast or sent to hand they're
//     out of the discover pile.
//  4. The misses (every exiled card EXCEPT the hit) are shuffled
//     and routed back to the bottom of the library via MoveCard so
//     §614 replacement chains and exile-leave triggers fire.
//
// Returns the hit card, or nil on whiff. The caller can inspect the
// event log for the actual disposition (discover_cast / discover_to_hand
// / discover_whiff).
func ApplyDiscoverWithChoice(gs *GameState, seatIdx, n int, choice DiscoverChoice) *Card {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || n < 0 {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	gs.LogEvent(Event{
		Kind:   "discover_trigger",
		Seat:   seatIdx,
		Amount: n,
		Details: map[string]interface{}{
			"rule": "701.51",
		},
	})

	exiled := make([]*Card, 0, 4)
	var hit *Card

	for len(seat.Library) > 0 {
		top := seat.Library[0]
		seat.Library = seat.Library[1:]
		if top == nil {
			continue
		}
		// Exile the card. Use MoveCard? No — the source already left
		// the library via slice-pop above; we mirror Cascade's pattern
		// of treating the in-flight pile as the exile cohort and only
		// invoking MoveCard for the eventual library-bottom return.
		// This keeps the temporary exile from spamming zone-change
		// events that aren't observable per CR §701.51 (discover
		// doesn't fire "card exiled" triggers on its way through).
		seat.Exile = append(seat.Exile, top)
		exiled = append(exiled, top)

		if discoverIsHit(top, n) {
			hit = top
			break
		}
	}

	if hit == nil {
		// Whiff — all exiled cards go to the library bottom.
		gs.LogEvent(Event{
			Kind:   "discover_whiff",
			Seat:   seatIdx,
			Amount: len(exiled),
			Details: map[string]interface{}{
				"discover_n":   n,
				"cards_exiled": len(exiled),
				"rule":         "701.51",
			},
		})
		returnDiscoverMissesToLibrary(gs, seatIdx, exiled, nil)
		return nil
	}

	// Hit found. Remove the hit from the exiled cohort so it doesn't
	// get returned with the misses.
	missesToBottom := exiled[:0]
	for _, c := range exiled {
		if c != hit {
			missesToBottom = append(missesToBottom, c)
		}
	}

	switch choice {
	case DiscoverChoiceToHand:
		// Pull from exile and put into hand via the canonical
		// zone-move path so any "card put into hand from exile"
		// triggers fire.
		MoveCard(gs, hit, seatIdx, "exile", "hand", "discover_to_hand")
		gs.LogEvent(Event{
			Kind:   "discover_to_hand",
			Seat:   seatIdx,
			Source: hit.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"discover_n":   n,
				"cards_exiled": len(exiled),
				"rule":         "701.51",
			},
		})

	default: // DiscoverChoiceCastFree
		// Register the ZoneCastPermission for the free cast from
		// exile. Cleared after the stack item is pushed.
		if gs.ZoneCastGrants == nil {
			gs.ZoneCastGrants = map[*Card]*ZoneCastPermission{}
		}
		gs.ZoneCastGrants[hit] = NewDiscoverCastFromExilePermission(seatIdx)

		// Build the stack item flagged as a discover cast.
		item := &StackItem{
			Card:       hit,
			Controller: seatIdx,
			CastZone:   ZoneExile,
			Effect:     collectSpellEffect(hit),
			CostMeta: map[string]interface{}{
				"discover_cast": true,
				"alt_cost":      "discover",
				"discover_n":    n,
				"free_cast":     true, // legacy cascade-shape compat
			},
		}
		PushStackItem(gs, item)

		// Permission consumed by the push — drop the grant so later
		// cast checks don't see a stale entry.
		delete(gs.ZoneCastGrants, hit)

		// Remove the hit from the exile zone (it's now on the stack
		// as a cast spell; the underlying Card pointer moved to the
		// stack item and shouldn't double-live in exile).
		removeFromZone(seat, hit, ZoneExile)

		gs.LogEvent(Event{
			Kind:   "discover_cast",
			Seat:   seatIdx,
			Source: hit.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"discover_n":   n,
				"cards_exiled": len(exiled),
				"rule":         "701.51",
			},
		})
	}

	returnDiscoverMissesToLibrary(gs, seatIdx, missesToBottom, hit)
	return hit
}

// discoverIsHit reports whether `c` qualifies as a discover hit for
// the given N: nonland AND CMC <= n.
func discoverIsHit(c *Card, n int) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		if strings.EqualFold(t, "land") {
			return false
		}
	}
	return c.CMC <= n
}

// returnDiscoverMissesToLibrary shuffles `misses` and routes each card
// from exile back to the library bottom via MoveCard. `hit` is passed
// only for the log-event source name (callers pass nil on whiff).
func returnDiscoverMissesToLibrary(gs *GameState, seatIdx int, misses []*Card, hit *Card) {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) || len(misses) == 0 {
		return
	}
	// CR §701.51: "Then put all other cards exiled this way on the
	// bottom of your library in a random order."
	if gs.Rng != nil && len(misses) > 1 {
		gs.Rng.Shuffle(len(misses), func(i, j int) {
			misses[i], misses[j] = misses[j], misses[i]
		})
	}
	for _, c := range misses {
		if c == nil {
			continue
		}
		MoveCard(gs, c, seatIdx, "exile", "library", "discover_bottom_of_library")
	}
	source := "discover"
	if hit != nil {
		source = hit.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "discover_bottom",
		Seat:   seatIdx,
		Source: source,
		Amount: len(misses),
		Details: map[string]interface{}{
			"cards_returned": len(misses),
			"rule":           "701.51",
		},
	})
}
