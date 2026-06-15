package gameengine

// keywords_omen.go — Omen (CR §702.176) as a real cast-from-hand alt-cost
// mechanic that exiles the spell on resolution and grants a one-time
// cast-from-exile permission for the "omen face."
//
// Omen appears on Foundations / Bloomburrow-style class cards. The
// printed rules text is effectively:
//
//   Omen [cost]
//   (Rather than cast this card from your hand, you may pay [cost] and
//    exile it with an omen counter. Activate only as a sorcery. Cast it
//    on a later turn from exile.)
//
// In rules-engine terms it is the same shape as Warp (§702.185) and
// Flashback (§702.34): an alternative cost (CR §601.2b / §601.2f-h)
// that flips an "exile instead of graveyard" replacement and produces a
// fresh ZoneCastPermission so the owner can cast the exiled face later
// at its printed mana cost.
//
// Wiring:
//   - HasOmen / OmenCost are thin keyword readers (parallel to
//     HasFlashback / FlashbackCost / HasMayhem / MayhemCost).
//   - CastOmen is the alt-cost cast entry point. It removes the card
//     from the caller's hand, pays the omen cost, pushes a StackItem
//     with CostMeta{"omen": true, "omen_cost": N, "exile_on_resolve":
//     true, "zone_cast_keyword": "omen"} so that stack.go's existing
//     ShouldExileOnResolve branch routes the card to exile on
//     resolution (CR §702.176, "exile it" clause), and registers a
//     ZoneCastPermission keyed to the card pointer with Zone=exile so
//     the owner may cast the omen face later (CR §702.176, "cast it on
//     a later turn from exile" clause). It also sets the seat-level
//     flag "spell_omen_cast_this_turn:<seat>" for any card that wants
//     to key off "if you cast a spell for its omen cost this turn."
//   - NewOmenCastFromExilePermission builds the post-resolve grant so
//     tests and per-card hooks can synthesize it directly.
//
// Sorcery-speed and zone-cast-restriction checks (Drannith Magistrate,
// generic §601.2a legality) are intentionally NOT enforced here; the
// upstream cast pipeline runs them before any alt-cost decision is
// made, matching CastFlashback / CastWarp / CastMayhem.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasOmen / OmenCost
// ---------------------------------------------------------------------------

// HasOmen reports whether the card has the omen keyword.
func HasOmen(card *Card) bool {
	return cardHasKeywordByName(card, "omen")
}

// OmenCost returns the converted mana cost of the omen keyword's
// alternative cost. The keyword arg may be a mana string ("{1}{G}") or
// a plain numeric value. Returns 0 if the keyword is absent or its args
// are malformed; callers should treat 0 as "free" only when they have
// positively confirmed HasOmen.
func OmenCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "omen") {
			continue
		}
		if len(kw.Args) == 0 {
			return card.CMC
		}
		switch v := kw.Args[0].(type) {
		case string:
			if cost, err := mana.Parse(v); err == nil {
				return cost.CMC()
			}
			return 0
		case float64:
			return int(v)
		case int:
			return v
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// CastOmen — pay the alt cost from hand, exile-on-resolve, grant exile cast.
// ---------------------------------------------------------------------------

// CastOmen casts a card from `seatIdx`'s hand for its omen cost.
// CR §702.176.
//
// Preconditions enforced here:
//   - card has the omen keyword
//   - card is in `seatIdx`'s hand
//   - seat can afford `omenCost` mana (pass -1 to use the printed OmenCost)
//
// On success the card is removed from the hand, mana is paid, and a
// StackItem is pushed with CostMeta:
//
//	{
//	  "omen":              true,
//	  "omen_cost":         N,
//	  "zone_cast_keyword": "omen",
//	  "exile_on_resolve":  true,
//	}
//
// The CostMeta["exile_on_resolve"] flag is the canonical hand-off to
// stack.go's ResolveStackTop branch (zone_cast.go ShouldExileOnResolve)
// which routes the card to exile after resolution. Simultaneously a
// ZoneCastPermission is registered on the card pointer so the omen face
// may be cast from exile later at the card's printed mana cost — see
// NewOmenCastFromExilePermission for the shape of that grant.
//
// The seat-level flag "spell_omen_cast_this_turn:<seat>" is set for any
// card that keys off "if you cast a spell for its omen cost this turn."
func CastOmen(gs *GameState, seatIdx int, card *Card, omenCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasOmen(card) {
		return nil, &CastError{Reason: "no_omen_keyword"}
	}
	if omenCost < 0 {
		omenCost = OmenCost(card)
	}
	if omenCost < 0 {
		return nil, &CastError{Reason: "invalid_omen_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the omen alternative cost
	// too (the omen is cast from hand). Zero-value context.
	omenCost = EffectiveAlternativeCost(gs, card, seatIdx, omenCost, CastContext{})
	if seat.ManaPool < omenCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	// Remove from hand.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the omen cost.
	seat.ManaPool -= omenCost
	SyncManaAfterSpend(seat)
	if omenCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: omenCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "omen_cast",
				"keyword": "omen",
				"rule":    "601.2f",
			},
		})
	}
	// Push onto the stack flagged for resolve-time exile.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"exile_on_resolve":  true,
			"zone_cast_keyword": "omen",
			"omen":              true,
			"omen_cost":         omenCost,
		},
	}
	PushStackItem(gs, item)

	// Mark the seat as having omen-cast a spell this turn — used by
	// cards/triggers that key off "if you cast a card for its omen cost
	// this turn." Cleared in cleanup with other "this turn" flags.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_omen_cast_this_turn:"+itoa(seatIdx)] = 1

	// Register the cast-from-exile permission keyed on the card pointer.
	// The grant has Zone=exile so it only activates once the spell has
	// resolved and the card has been moved into the owner's exile by the
	// ShouldExileOnResolve branch in stack.go. RemoveZoneCastGrant will
	// be invoked by the future from-exile cast (or by any LTB cleanup
	// that removes the card from exile).
	RegisterZoneCastGrant(gs, card, NewOmenCastFromExilePermission(card.Owner))

	gs.LogEvent(Event{
		Kind:   "omen_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: omenCost,
		Details: map[string]interface{}{
			"rule": "702.176",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellOmenCastThisTurn returns true if any spell was cast for its
// omen cost by `seatIdx` during the current turn.
func SpellOmenCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_omen_cast_this_turn:"+itoa(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// NewOmenCastFromExilePermission — the post-resolve "cast from exile" grant.
// ---------------------------------------------------------------------------

// NewOmenCastFromExilePermission returns a ZoneCastPermission that lets
// the card's owner cast the omen face from exile at the card's normal
// mana cost. CR §702.176 ("cast it on a later turn from exile") — the
// alt omen cost itself is consumed by the first cast, so the second
// cast pays the printed mana cost. ManaCost = -1 instructs CastFromZone
// / CanCastFromZone to use the card's printed mana cost.
//
// RequireController is the owner so an opponent that exiled the card
// (e.g. via a steal effect resolving on the omen-cast spell) cannot
// hijack the second cast. Duration is empty — the grant persists until
// the card leaves exile (cast or removed) per CR §702.176.
func NewOmenCastFromExilePermission(owner int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "omen",
		ManaCost:          -1, // use card's printed mana cost
		RequireController: owner,
		SourceName:        "omen_exile",
		Duration:          "", // permanent until cast or otherwise removed
	}
}
