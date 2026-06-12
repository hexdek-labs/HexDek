package gameengine

import "strings"

// keywords_partner.go — round-26 surface wrappers + ETB hook for the
// partner-family commander keywords.
//
// CR §702.124  — bare Partner
// CR §702.124g — Partner with [name]
// CR §702.131  — Friends Forever (Commander Legends — Battle for Baldur's Gate)
// CR §702.139  — Doctor's Companion (the version this file does NOT
//                touch — see ReadPartnerInfo for the Doctor case).
//
// The PartnerInfo parser + ValidatePartnerPair legality checker already
// live in multiplayer.go; this file is the *card-facing* surface (the
// predicates / target lookups card resolvers and per_card handlers
// reach for) plus the §702.124g ETB search-and-grab hook. Where this
// file's helpers wrap something in multiplayer.go we delegate rather
// than re-implement to keep both surfaces consistent.

// ---------------------------------------------------------------------------
// Predicates
// ---------------------------------------------------------------------------

// HasFriendsForever reports whether the card has the Friends Forever
// keyword (CR §702.131). Friends Forever is functionally identical to
// Partner for commander-pair legality but is its OWN keyword — a
// Friends-Forever card cannot pair with a bare-Partner card.
func HasFriendsForever(card *Card) bool {
	return ReadPartnerInfo(card).FriendsForever
}

// HasPartnerWith reports whether the card has the "Partner with [name]"
// keyword (CR §702.124g). True only for the named-pair variant, NOT
// for bare Partner; callers wanting any partner-family keyword should
// use ReadPartnerInfo(card).HasPartner().
func HasPartnerWith(card *Card) bool {
	return ReadPartnerInfo(card).PartnerWith != ""
}

// PartnerWithTarget returns the printed partner name for a "Partner
// with X" card. Returns "" for cards without the keyword. Use this
// when generating the ETB search-and-grab trigger or when validating
// a specific deck's pairing.
func PartnerWithTarget(card *Card) string {
	return ReadPartnerInfo(card).PartnerWith
}

// IsBackground returns true if the card's type line includes the
// "Background" subtype (CR §702.124e — Background is an enchantment
// subtype introduced in CLB). Backgrounds pair with commanders that
// have "Choose a Background"; they are NOT themselves commanders
// unless their controller's deck pairs one with a chooser.
//
// Detection delegates to cardHasSubtype, which checks both Card.Types
// (canonical for runtime cards) and Card.TypeLine (fallback for
// tokens / corpus-loaded cards that haven't been re-parsed).
func IsBackground(card *Card) bool {
	return cardHasSubtype(card, "background")
}

// HasChooseABackground returns true if the card carries the "Choose a
// Background" commander keyword (CR §702.131 — CLB legendary creatures
// like Wilson, Refined Grizzly). Cards with this keyword may be paired
// with any Background-subtype enchantment as a second commander.
func HasChooseABackground(card *Card) bool {
	return ReadPartnerInfo(card).ChooseBackground
}

// ---------------------------------------------------------------------------
// Pair-legality wrapper
// ---------------------------------------------------------------------------

// CanBeCommandersTogether returns true if `a` and `b` form a legal
// commander pair under CR §702.124 / §702.131 / §903.3c. Boolean
// wrapper around ValidatePartnerPair, kept here so card-facing code
// reads naturally:
//
//	if CanBeCommandersTogether(thrasios, vial) { ... }
//
// Valid configurations (mirrors ValidatePartnerPair, restated for the
// card-facing audience):
//
//  1. Both have bare Partner
//  2. a's "Partner with X" names b AND b's "Partner with X" names a
//  3. Both have Friends Forever
//  4. One has Choose a Background, the other is a Background
//  5. One is a Doctor, the other has Doctor's Companion
//
// Cross-category mixes (Friends Forever + bare Partner, Partner with X
// vs. unrelated partner card, etc.) are rejected.
//
// The Background pairing (case 4) is wired explicitly at the card-
// facing surface so it stays auditable from this file alone — callers
// reading keywords_partner.go don't have to chase the rule through
// the multiplayer.go legality validator. The remaining four cases
// delegate to ValidatePartnerPair, the canonical implementation.
func CanBeCommandersTogether(a, b *Card) bool {
	if a == nil || b == nil {
		return false
	}
	// §702.124e — Choose a Background pairs with any single Background.
	// Order-independent: either side may be the chooser.
	if HasChooseABackground(a) && IsBackground(b) {
		return true
	}
	if HasChooseABackground(b) && IsBackground(a) {
		return true
	}
	return ValidatePartnerPair([]*Card{a, b}) == nil
}

// ---------------------------------------------------------------------------
// ETB trigger — CR §702.124g
// ---------------------------------------------------------------------------
//
// "When this creature enters the battlefield, target player may search
//  their library for a card named [name], reveal it, put it into their
//  hand, then shuffle."
//
// Our simulation models the "target player" as the controller (the
// rational choice — fetching your own partner is what every actual
// deck does). The OnPartnerWithETB hook is wired by per_card / ETB-
// dispatch code at the point a permanent enters; we don't auto-fire
// from MoveCard so callers retain ordering control with other ETB
// triggers.

// OnPartnerWithETB executes the §702.124g enters-the-battlefield
// trigger for the permanent at hand. Walks the controller's library
// for a card whose name matches the partner-with target; if found,
// moves it to the controller's hand and shuffles the remaining
// library. No-op when:
//
//   - perm or perm.Card is nil
//   - the card lacks "Partner with"
//   - the controller seat is invalid
//   - the named partner isn't anywhere in the library (the may-search
//     succeeds vacuously per the printed rules — we just don't move
//     a card)
//
// Returns the matched card (or nil) so callers / tests can verify
// what was tutored.
func OnPartnerWithETB(gs *GameState, perm *Permanent) *Card {
	if gs == nil || perm == nil || perm.Card == nil {
		return nil
	}
	target := PartnerWithTarget(perm.Card)
	if target == "" {
		return nil
	}
	seatIdx := perm.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil
	}

	gs.LogEvent(Event{
		Kind:   "partner_with_search",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"rule":          "702.124g",
			"partner_named": target,
		},
	})

	var found *Card
	for _, c := range seat.Library {
		if c == nil {
			continue
		}
		if partnerNameMatch(target, c.DisplayName()) {
			found = c
			break
		}
	}
	if found != nil {
		gs.LogEvent(Event{
			Kind:   "reveal",
			Seat:   seatIdx,
			Source: found.DisplayName(),
			Details: map[string]interface{}{
				"reason": "partner_with",
				"rule":   "702.124g",
			},
		})
		MoveCard(gs, found, seatIdx, ZoneLibrary, ZoneHand, "partner_with")
	} else {
		gs.LogEvent(Event{
			Kind:   "partner_with_no_match",
			Seat:   seatIdx,
			Source: perm.Card.DisplayName(),
			Details: map[string]interface{}{
				"rule":          "702.124g",
				"partner_named": target,
			},
		})
	}

	// Per §702.124g the library is always shuffled, regardless of
	// whether the search succeeded.
	if gs.Rng != nil && len(seat.Library) > 1 {
		gs.Rng.Shuffle(len(seat.Library), func(i, j int) {
			seat.Library[i], seat.Library[j] = seat.Library[j], seat.Library[i]
		})
	}
	gs.LogEvent(Event{
		Kind:   "library_shuffled",
		Seat:   seatIdx,
		Source: perm.Card.DisplayName(),
		Details: map[string]interface{}{
			"reason": "partner_with",
			"rule":   "702.124g",
		},
	})

	return found
}

// ---------------------------------------------------------------------------
// Helper: case-insensitive name match for callers outside multiplayer.go
// ---------------------------------------------------------------------------

// PartnerNameMatch normalises a "Partner with X" reference and a
// candidate commander display name for comparison. Exported wrapper
// around the (unexported) partnerNameMatch in multiplayer.go so
// per_card / deckparser callers don't have to live in this package's
// internal namespace. Case-fold + trim.
func PartnerNameMatch(partnerWith, candidate string) bool {
	return strings.EqualFold(strings.TrimSpace(partnerWith),
		strings.TrimSpace(candidate))
}
