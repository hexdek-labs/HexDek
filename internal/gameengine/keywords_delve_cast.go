package gameengine

// keywords_delve_cast.go — Delve cast helper (CR §702.66).
//
// CR §702.66a: Delve is a static ability that functions while the spell
//              is on the stack. "Delve" means "For each generic mana in
//              this spell's total cost, you may exile a card from your
//              graveyard rather than pay that mana." Each card exiled
//              this way pays for {1} of the spell's generic mana cost.
//
// Companion helpers (already in keywords_misc.go):
//   - HasDelve(card) bool
//   - DelveMaxReduction(gs, seat) int   — = len(graveyard)
//   - PayDelve(gs, seat, card, max) int — exiles up to `max` and returns
//                                          the count actually exiled
//
// What this file adds is the cast-level wrapper that mirrors
// CastWithConvoke / CastBuyback / CastWithMadness:
//   - validates HasDelve, hand-presence, graveyard size, and the
//     declared delve count
//   - exiles `delveCount` cards from graveyard
//   - pays the net (normalCost - delveCount) generic mana
//   - pushes a StackItem with CostMeta["alt_cost"]="delve" so
//     downstream observers (Heimdall replay, Muninn parity) can
//     identify delve casts the same way they identify convoke/buyback.
//
// The caller decides `delveCount` — exactly the same shape as Convoke
// (caller passes `tappedCreatures`) and Buyback (caller passes both
// cost arms). Hat-side selection of delveCount is not modeled here;
// the auto-cast Hat does not yet choose to delve.

// CastWithDelve casts `card` from `seatIdx`'s hand paying `delveCount`
// generic mana via graveyard exile and the remainder out of the typed
// mana pool.
//
// Precondition checks (no mutation on failure):
//   - gs / seat / card non-nil
//   - card has the delve keyword
//   - delveCount in [0, min(normalCost, len(graveyard))]
//   - normalCost == card.CMC; net = normalCost - delveCount; seat
//     ManaPool >= net
//
// On success the cards are exiled most-recently-added first (matching
// the order PayDelve uses), the StackItem is pushed, and an event of
// kind "delve_cast" + a per-exile "delve" event (already fired by
// PayDelve) trace the operation.
func CastWithDelve(gs *GameState, seatIdx int, card *Card, delveCount int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil_game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid_seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil_card"}
	}
	if !HasDelve(card) {
		return nil, &CastError{Reason: "no_delve_keyword"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil_seat"}
	}

	// Cost arithmetic — validate before any mutation.
	normalCost := card.CMC
	if normalCost < 0 {
		normalCost = 0
	}
	if delveCount < 0 {
		return nil, &CastError{Reason: "negative_delve_count"}
	}
	if delveCount > normalCost {
		// Per §702.66a delve pays for GENERIC mana only; trying to
		// pay more than the generic portion is an illegal cost.
		return nil, &CastError{Reason: "delve_exceeds_generic_cost"}
	}
	if delveCount > len(seat.Graveyard) {
		return nil, &CastError{Reason: "delve_exceeds_graveyard_size"}
	}
	net := normalCost - delveCount
	if seat.ManaPool < net {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// Commit: remove from hand, exile delve cards, pay mana, push stack.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	exiled := PayDelve(gs, seatIdx, card, delveCount)
	if exiled != delveCount {
		// Defensive: graveyard shrunk between the pre-check and the
		// payment (a same-stack-frame side-effect could in principle
		// remove cards). Roll back the hand removal so the cast is
		// atomic-on-failure.
		seat.Hand = append(seat.Hand, card)
		return nil, &CastError{Reason: "delve_payment_short"}
	}
	if net > 0 {
		seat.ManaPool -= net
		SyncManaAfterSpend(seat)
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: net,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":      "delve_cast",
				"keyword":     "delve",
				"rule":        "702.66",
				"normal_cost": normalCost,
				"reduction":   delveCount,
			},
		})
	}

	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"alt_cost":        "delve",
			"delve_exiled":    delveCount,
			"delve_reduction": delveCount,
			"delve_net_cost":  net,
		},
	}
	PushStackItem(gs, item)

	gs.LogEvent(Event{
		Kind:   "delve_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: delveCount,
		Details: map[string]interface{}{
			"rule":        "702.66",
			"normal_cost": normalCost,
			"reduction":   delveCount,
			"net_paid":    net,
		},
	})
	return &CostPaymentResult{}, nil
}

// IsDelveCast reports whether the StackItem was cast via delve. Mirrors
// IsConvokeCast / IsBuybackCast / IsMadnessCast for symmetry — Heimdall
// and Muninn use these predicates to label replays.
func IsDelveCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	alt, _ := item.CostMeta["alt_cost"].(string)
	return alt == "delve"
}
