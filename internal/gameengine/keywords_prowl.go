package gameengine

// keywords_prowl.go — Prowl (CR §702.74, Lorwyn / Morningtide 2008).
//
// CR §702.74a: Prowl is a keyword that represents an alternative cost.
//               "Prowl [cost]" means "You may cast this spell by paying
//               [cost] rather than paying its mana cost if you dealt
//               combat damage to a player this turn with a creature
//               that shares a creature type with this spell."
// CR §702.74b: Casting a spell using its prowl ability follows the
//               rules for paying alternative costs in §601.2b and
//               §601.2f-h.
//
// Engine model
// ------------
// Prowl is a hand-cast alt cost gated on a "this turn" historical
// fact: did one of YOUR creatures with a shared subtype hit ANY
// player in combat this turn? The historical fact is tracked in
// seat.Turn.CombatDamageBy []*Card (populated by
// applyCombatDamageToPlayer), which survives the creature dying or
// being bounced after the damage event — Prowl asks about history,
// not current board state.
//
// Wiring:
//   - HasProwl / ProwlCost are thin keyword readers.
//   - CanPayProwl scans the active seat's Turn.CombatDamageBy slice
//     and returns true iff any card in it shares a creature subtype
//     with the prowl spell. Subtype comparison filters out
//     supertypes (legendary, snow, world) and the base "creature"
//     card type — only proper subtypes like Rogue, Goblin, Faerie
//     count, matching the existing CanCastForProwl reader in
//     keywords_batch4.go (which scans current battlefield instead;
//     this file's reader is the historically-correct version).
//   - CastWithProwl validates HasProwl + CanPayProwl, removes the
//     card from hand, pays the prowl alt cost, pushes a StackItem
//     stamped CostMeta{"prowl_cast": true, "prowl_cost": N}. Sets
//     the seat-level flag spell_prowled_this_turn:<seat> for any
//     "if you cast a spell for its prowl cost this turn" trigger.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasProwl / ProwlCost
// ---------------------------------------------------------------------------

// HasProwl reports whether the card has the prowl keyword.
func HasProwl(card *Card) bool {
	return cardHasKeywordByName(card, "prowl")
}

// ProwlCost returns the converted mana cost of the prowl keyword's
// alternative cost. Accepts the keyword arg as either a mana string
// ("{B}") or a plain numeric value. Returns 0 if the keyword is
// absent or the args are malformed; callers should treat 0 as "free"
// only when they have positively confirmed HasProwl.
func ProwlCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "prowl") {
			continue
		}
		if len(kw.Args) == 0 {
			return card.CMC
		}
		if cost := parseManaOrNumericArg(kw.Args[0]); cost >= 0 {
			return cost
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// CanPayProwl — historical "shared subtype hit a player" predicate
// ---------------------------------------------------------------------------

// CanPayProwl reports whether the prowl precondition for `card` is
// currently satisfied for `seatIdx`: at least one card in
// seat.Turn.CombatDamageBy must share a creature subtype with `card`
// (per CR §702.74a). Supertypes (legendary, snow, world) and the
// base "creature" type are excluded from the comparison — Prowl
// keys on creature subtypes, not card types.
//
// Returns false when the spell or seat is invalid, when the spell
// has no creature subtypes, when nobody dealt combat damage to a
// player this turn, or when no overlap exists.
func CanPayProwl(gs *GameState, seatIdx int, card *Card) bool {
	if gs == nil || card == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return false
	}
	if len(seat.Turn.CombatDamageBy) == 0 {
		return false
	}
	cardSubs := creatureSubtypesOf(card)
	if len(cardSubs) == 0 {
		return false
	}
	for _, dealer := range seat.Turn.CombatDamageBy {
		if dealer == nil {
			continue
		}
		for sub := range creatureSubtypesOf(dealer) {
			if _, ok := cardSubs[sub]; ok {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// CastWithProwl
// ---------------------------------------------------------------------------

// CastWithProwl casts a card from `seatIdx`'s hand for its prowl
// alternative cost. CR §702.74a.
//
// Preconditions enforced here:
//   - card has the prowl keyword (HasProwl)
//   - CanPayProwl(gs, seatIdx, card) is true at cast time
//   - card is in seat's hand
//   - seat can afford `prowlCost` mana (pass -1 to use the printed
//     ProwlCost)
//
// On success: card removed from hand, prowl cost paid (the alt
// cost, NOT the printed mana cost), StackItem pushed with
// CostMeta{"prowl_cast": true, "prowl_cost": N}, seat-level flag
// spell_prowled_this_turn:<seat> set. Prowl does NOT change
// resolution destination, so no exile_on_resolve / return_to_hand
// flags are stamped on the StackItem.
func CastWithProwl(gs *GameState, seatIdx int, card *Card, prowlCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasProwl(card) {
		return nil, &CastError{Reason: "no_prowl_keyword"}
	}
	if !CanPayProwl(gs, seatIdx, card) {
		return nil, &CastError{Reason: "prowl_not_active"}
	}
	if prowlCost < 0 {
		prowlCost = ProwlCost(card)
	}
	if prowlCost < 0 {
		return nil, &CastError{Reason: "invalid_prowl_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the prowl alternative
	// cost too (cast from hand).
	prowlCost = EffectiveAlternativeCost(gs, card, seatIdx, prowlCost, CastContext{})
	if seat.ManaPool < prowlCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the prowl alt cost (not the printed mana cost — §702.74a
	// "rather than paying its mana cost").
	seat.ManaPool -= prowlCost
	SyncManaAfterSpend(seat)
	if prowlCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: prowlCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "prowl_cast",
				"keyword": "prowl",
				"rule":    "601.2f",
			},
		})
	}
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"prowl_cast": true,
			"prowl_cost": prowlCost,
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_prowled_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "prowl_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: prowlCost,
		Details: map[string]interface{}{
			"rule": "702.74a",
		},
	})
	return &CostPaymentResult{}, nil
}

// ---------------------------------------------------------------------------
// Stack / per-turn predicates
// ---------------------------------------------------------------------------

// IsProwlCast reports whether a StackItem was cast for its prowl cost.
func IsProwlCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["prowl_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellProwledThisTurn returns true if any spell was cast via prowl
// by `seatIdx` during the current turn.
func SpellProwledThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_prowled_this_turn:"+itoa(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// creatureSubtypesOf returns the set of creature subtypes on a card
// (Rogue, Goblin, Faerie, etc.), excluding supertypes (legendary,
// snow, world) and the base "creature" type token. Names are
// lower-cased so the lookup is case-insensitive.
//
// Returns an empty map for nil cards or cards with no qualifying
// subtypes.
func creatureSubtypesOf(card *Card) map[string]struct{} {
	out := map[string]struct{}{}
	if card == nil {
		return out
	}
	for _, t := range card.Types {
		low := strings.ToLower(strings.TrimSpace(t))
		switch low {
		case "", "creature", "token", "legendary", "snow", "world",
			"basic", "tribal", "instant", "sorcery", "artifact",
			"enchantment", "planeswalker", "land", "battle":
			continue
		}
		out[low] = struct{}{}
	}
	return out
}

// parseManaOrNumericArg parses a keyword arg value (string for mana
// notation, int / float64 for plain numeric) into a converted-mana
// integer. Returns -1 on parse failure so the caller can distinguish
// "absent / malformed" from "validly zero." Mirrors the arg-shape
// expected by FlashbackCost / MayhemCost / OmenCost / EscapeCost.
func parseManaOrNumericArg(arg any) int {
	switch v := arg.(type) {
	case string:
		if cost, err := mana.Parse(v); err == nil {
			return cost.CMC()
		}
		return -1
	case float64:
		return int(v)
	case int:
		return v
	}
	return -1
}
