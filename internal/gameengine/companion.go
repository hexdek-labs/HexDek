package gameengine

import "strings"

// Companion mechanic (CR §702.139).
//
// Pre-game: a player may reveal a companion card from outside the game
// if their deck satisfies the companion restriction.
//
// During the game: once per game, any time the player could cast a
// sorcery (main phase, empty stack, their turn), they may pay {3} to
// put the companion from outside the game into their hand.
//
// This file provides:
//   - DeclareCompanion(gs, seat, card) — pre-game companion declaration
//   - MoveCompanionToHand(gs, seat) — pay {3}, move companion to hand
//   - CheckCompanionRestriction(name, deck) — verify deck meets restriction

// DeclareCompanion sets the given card as seat's companion. Call during
// pre-game setup (after mulligan resolution, before the first turn).
// The companion card is stored in Seat.Companion and does NOT start in
// the library or hand.
func DeclareCompanion(gs *GameState, seatIdx int, card *Card) {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return
	}
	seat.Companion = card
	gs.LogEvent(Event{
		Kind:   "companion_declared",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.139",
		},
	})
}

// MoveCompanionToHand pays {3} and moves the companion card from outside
// the game into the seat's hand. Returns nil on success, an error string
// on failure.
func MoveCompanionToHand(gs *GameState, seatIdx int) error {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return &CastError{Reason: "invalid_state"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return &CastError{Reason: "nil_seat"}
	}
	if seat.Companion == nil {
		return &CastError{Reason: "no_companion"}
	}
	if seat.CompanionMoved {
		return &CastError{Reason: "companion_already_moved"}
	}
	// Must be able to pay {3}.
	const companionTax = 3
	if seat.ManaPool < companionTax {
		return &CastError{Reason: "insufficient_mana_for_companion"}
	}
	// Can only do this at sorcery speed (main phase, empty stack, your turn).
	if gs.Active != seatIdx {
		return &CastError{Reason: "not_your_turn"}
	}
	if len(gs.Stack) > 0 {
		return &CastError{Reason: "stack_not_empty"}
	}

	// Pay the tax.
	seat.ManaPool -= companionTax
	SyncManaAfterSpend(seat)
	gs.LogEvent(Event{
		Kind:   "pay_mana",
		Seat:   seatIdx,
		Amount: companionTax,
		Source: seat.Companion.DisplayName(),
		Details: map[string]interface{}{
			"reason": "companion_tax",
			"rule":   "702.139",
		},
	})

	// Move to hand.
	MoveCard(gs, seat.Companion, seatIdx, "companion", "hand", "companion-to-hand")
	seat.CompanionMoved = true
	gs.LogEvent(Event{
		Kind:   "companion_to_hand",
		Seat:   seatIdx,
		Source: seat.Companion.DisplayName(),
		Details: map[string]interface{}{
			"rule": "702.139",
		},
	})

	return nil
}

// CheckCompanionRestriction verifies that a deck meets the deckbuilding
// restriction imposed by a given companion card. Returns true if the
// restriction is satisfied. Unknown companions are assumed valid.
//
// Supported companions:
//   - Lurrus of the Dream-Den — each permanent card has MV 2 or less
//   - Yorion, Sky Nomad — deck has 80+ cards (always true in Commander)
//   - Obosh, the Preypiercer — each nonland card has odd MV
//   - Gyruda, Doom of Depths — each nonland card has even MV
//   - Kaheera, the Orphanguard — each creature is Cat/Elemental/Nightmare/Dinosaur/Beast
//   - Jegantha, the Wellspring — no card has more than one of the same mana symbol
//   - Keruga, the Macrosage — each nonland card has MV 3 or greater
//   - Umori, the Collector — each nonland card shares a card type (simplified: always pass)
//   - Zirda, the Dawnwaker — each permanent has an activated ability (simplified: always pass)
//   - Lutri, the Spellchaser — no nonland card appears more than once (singleton)
func CheckCompanionRestriction(companionName string, deck []*Card) bool {
	name := strings.ToLower(companionName)
	switch {
	case strings.Contains(name, "lurrus"):
		// Each permanent card has MV 2 or less.
		for _, c := range deck {
			if companionCardIsPermanent(c) && c.CMC > 2 {
				return false
			}
		}
		return true
	case strings.Contains(name, "yorion"):
		// Deck has 80+ cards.
		return len(deck) >= 80
	case strings.Contains(name, "obosh"):
		// Each nonland card has odd MV.
		for _, c := range deck {
			if !cardHasType(c, "land") && c.CMC%2 == 0 {
				return false
			}
		}
		return true
	case strings.Contains(name, "gyruda"):
		// Each nonland card has even MV.
		for _, c := range deck {
			if !cardHasType(c, "land") && c.CMC%2 != 0 {
				return false
			}
		}
		return true
	case strings.Contains(name, "kaheera"):
		// Each creature is Cat, Elemental, Nightmare, Dinosaur, or Beast.
		allowedTypes := map[string]bool{
			"cat": true, "elemental": true, "nightmare": true,
			"dinosaur": true, "beast": true,
		}
		for _, c := range deck {
			if !cardHasType(c, "creature") {
				continue
			}
			found := false
			for _, t := range c.Types {
				if allowedTypes[strings.ToLower(t)] {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case strings.Contains(name, "keruga"):
		// Each nonland card has MV 3 or greater.
		for _, c := range deck {
			if !cardHasType(c, "land") && c.CMC < 3 {
				return false
			}
		}
		return true
	case strings.Contains(name, "lutri"):
		// No nonland card appears more than once (singleton).
		seen := map[string]bool{}
		for _, c := range deck {
			if cardHasType(c, "land") {
				continue
			}
			n := ""
			if c != nil {
				n = c.DisplayName()
			}
			if seen[n] {
				return false
			}
			seen[n] = true
		}
		return true
	default:
		// Unknown companion or simplified (Umori, Zirda, Jegantha) — assume valid.
		return true
	}
}

// companionCardIsPermanent returns true if the card is a permanent type.
func companionCardIsPermanent(c *Card) bool {
	if c == nil {
		return false
	}
	for _, t := range c.Types {
		tl := strings.ToLower(t)
		if tl == "creature" || tl == "artifact" || tl == "enchantment" ||
			tl == "planeswalker" || tl == "land" || tl == "battle" {
			return true
		}
	}
	return false
}
