package gameengine

// keywords_buyback.go — Buyback (CR §702.27) as a real alt-cost mechanic.
//
// CR §702.27a: Buyback appears on some instants and sorceries. It represents
//              two static abilities that function while the spell is on the
//              stack. "Buyback [cost]" means "You may pay an additional [cost]
//              as you cast this spell" and "If the buyback cost was paid, put
//              this spell into its owner's hand instead of into that player's
//              graveyard as it resolves."
//
// Implementation:
//
//   - HasBuyback(card) reports the keyword.
//   - BuybackCost(card) parses the keyword arg's converted mana cost
//     (matches the FlashbackCost / WarpCost pattern — mana strings or a
//     plain numeric arg).
//   - CastBuyback runs the §601.2f "additional cost" path: it pays the
//     spell's normal mana cost PLUS the buyback cost, removes the card
//     from hand, and pushes a StackItem flagged with
//     CostMeta["bought_back"]=true.
//   - ShouldReturnToHandOnResolve is the predicate consumed by
//     ResolveStackTop in stack.go — when true, the resolving non-permanent
//     spell is routed to its owner's hand instead of the graveyard per
//     §702.27a.
//
// Scope notes:
//
//   - Buyback is, in printed practice, always a mana cost — the engine
//     models it as an integer mana amount (matches Warp/Kicker). Buyback
//     riders that ask for non-mana payment (none exist on printed cards;
//     this is a guard against future custom corpora) are not modeled.
//   - §702.27a restricts buyback to instants/sorceries; CastBuyback
//     enforces that as defense-in-depth in case a corpus mistype tags a
//     permanent with the keyword.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasBuyback / BuybackCost
// ---------------------------------------------------------------------------

// HasBuyback returns true if the card has the buyback keyword in its AST.
func HasBuyback(card *Card) bool {
	return cardHasKeywordByName(card, "buyback")
}

// BuybackCost returns the converted mana cost of the buyback keyword's
// additional cost. Accepts the keyword arg as either a mana string
// ("{1}{u}") or a plain numeric value. Returns 0 if the keyword is absent
// or the args are malformed; callers should treat 0 as "free" only when
// they have positively confirmed HasBuyback.
func BuybackCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "buyback") {
			continue
		}
		if len(kw.Args) == 0 {
			return 0
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
// Stack predicates
// ---------------------------------------------------------------------------

// IsBoughtBack reports whether a StackItem carries the bought-back flag.
func IsBoughtBack(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["bought_back"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// ShouldReturnToHandOnResolve returns true when the resolving non-permanent
// spell should be routed to its owner's hand instead of the graveyard.
// CR §702.27a. Consumed by ResolveStackTop in stack.go.
func ShouldReturnToHandOnResolve(item *StackItem) bool {
	return IsBoughtBack(item)
}

// ---------------------------------------------------------------------------
// CastBuyback
// ---------------------------------------------------------------------------

// CastBuyback casts a card from `seatIdx`'s hand for its normal mana cost
// PLUS the buyback cost. CR §702.27a — buyback is an ADDITIONAL cost
// (§601.2f), not an alternative one; the caller passes both `normalCost`
// (the card's printed mana cost the spell would otherwise pay) and
// `buybackCost` (the keyword's parsed additional cost) so the arithmetic
// is explicit and easy to test in isolation from the full cast pipeline.
//
// Preconditions:
//   - card is in seat's hand
//   - card has the buyback keyword
//   - card type is instant or sorcery (CR §702.27a — buyback only appears
//     on instants/sorceries)
//   - seat can afford normalCost + buybackCost
//   - normal timing/legality applies (sorcery-speed for sorceries, etc.) —
//     checked by upstream cast pipeline; CastBuyback itself does not
//     enforce sorcery-speed because that's a generic spell-casting rule
//     enforced before any alt-cost decision.
//
// On success: card removed from hand, both costs paid, StackItem pushed
// with CostMeta["bought_back"]=true and CostMeta["buyback_cost"]=buybackCost.
// Seat flag "spell_bought_back_this_turn:<seat>" is set for any future card
// that keys off "if you cast a spell with buyback this turn." Cleared in
// cleanup alongside other "this turn" flags (caller responsibility; we
// just set the marker).
func CastBuyback(gs *GameState, seatIdx int, card *Card, normalCost int, buybackCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasBuyback(card) {
		return nil, &CastError{Reason: "no_buyback_keyword"}
	}
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		return nil, &CastError{Reason: "buyback_not_instant_or_sorcery"}
	}
	if normalCost < 0 {
		return nil, &CastError{Reason: "invalid_normal_cost"}
	}
	if buybackCost < 0 {
		return nil, &CastError{Reason: "invalid_buyback_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	total := normalCost + buybackCost
	if seat.ManaPool < total {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	seat.ManaPool -= total
	SyncManaAfterSpend(seat)
	if total > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: total,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":       "buyback_cast",
				"keyword":      "buyback",
				"rule":         "601.2f",
				"normal_cost":  normalCost,
				"buyback_cost": buybackCost,
			},
		})
	}
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"bought_back":  true,
			"buyback_cost": buybackCost,
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_bought_back_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "buyback_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: buybackCost,
		Details: map[string]interface{}{
			"rule": "702.27a",
		},
	})
	return &CostPaymentResult{}, nil
}
