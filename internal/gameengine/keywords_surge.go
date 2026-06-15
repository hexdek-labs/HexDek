package gameengine

// keywords_surge.go — Surge (CR §702.117) cast helper.
//
// CR §702.117a: Surge is a keyword that represents an alternative cost.
//               "Surge [cost]" means "You may cast this spell by paying
//               [cost] rather than paying its mana cost if you or a
//               teammate has cast another spell this turn."
// CR §702.117b: Casting a spell using its surge ability follows the
//               rules for paying alternative costs in §601.2b and
//               §601.2f-h.
//
// The eligibility predicate, keyword reader, and cost reader already
// live in keywords_combat.go (HasSurge / SurgeCost / CanPaySurge); this
// file adds CastWithSurge, the alt-cost cast entry point that mirrors
// CastFlashback / CastWarp / CastOmen.
//
// Wiring:
//   - HasSurge / SurgeCost / CanPaySurge — see keywords_combat.go.
//   - CastWithSurge removes the card from the caller's hand, pays the
//     surge cost (the alt cost — NOT the printed mana cost), and pushes
//     a StackItem stamped with CostMeta{"surge_cast": true,
//     "surge_cost": N}. Surge does not change resolution destination,
//     so no exile_on_resolve / return_to_hand flags are set; the spell
//     resolves to its normal post-resolve zone (CR §608.2g for
//     non-permanent, battlefield for permanent).
//   - SpellSurgedThisTurn exposes the per-turn "did seat cast a surge
//     spell this turn" flag for any card that wants to key off it.
//
// Scope note on "you or a teammate":
//   - CanPaySurge currently checks seat.Turn.SpellsCast > 0, which
//     covers the four-player Commander default where every player is
//     their own team. Two-Headed Giant team partitioning is not
//     modeled. CastWithSurge defers entirely to CanPaySurge for that
//     decision so a future expansion (when team-mode lands) lights up
//     here automatically.

// ---------------------------------------------------------------------------
// CastWithSurge
// ---------------------------------------------------------------------------

// CastWithSurge casts a card from `seatIdx`'s hand using its surge
// alternative cost. CR §702.117a.
//
// Preconditions enforced here:
//   - card has the surge keyword (HasSurge)
//   - the surge precondition holds (CanPaySurge — "you or a teammate
//     has cast another spell this turn")
//   - card is in seat's hand
//   - seat can afford `surgeCost` mana (pass -1 to use the printed
//     SurgeCost)
//
// On success the card is removed from hand, the surge cost is paid in
// full (this is the alt cost, paid instead of the printed mana cost),
// and a StackItem is pushed with CostMeta:
//
//	{
//	  "surge_cast": true,
//	  "surge_cost": N,
//	}
//
// The seat-level flag "spell_surged_this_turn:<seat>" is set for any
// card or trigger that keys off "if you cast a spell for its surge
// cost this turn."
//
// Surge does NOT alter the resolution destination — the spell resolves
// to its normal post-resolve zone, so no exile_on_resolve or
// return_to_hand flag is stamped on the StackItem.
func CastWithSurge(gs *GameState, seatIdx int, card *Card, surgeCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasSurge(card) {
		return nil, &CastError{Reason: "no_surge_keyword"}
	}
	if !CanPaySurge(gs, seatIdx) {
		return nil, &CastError{Reason: "surge_not_active"}
	}
	if surgeCost < 0 {
		surgeCost = SurgeCost(card)
	}
	if surgeCost < 0 {
		return nil, &CastError{Reason: "invalid_surge_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the surge alternative
	// cost too (cast from hand).
	surgeCost = EffectiveAlternativeCost(gs, card, seatIdx, surgeCost, CastContext{})
	if seat.ManaPool < surgeCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the surge alt cost (not the printed mana cost — §702.117a
	// "rather than paying its mana cost").
	seat.ManaPool -= surgeCost
	SyncManaAfterSpend(seat)
	if surgeCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: surgeCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "surge_cast",
				"keyword": "surge",
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
			"surge_cast": true,
			"surge_cost": surgeCost,
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_surged_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "surge_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: surgeCost,
		Details: map[string]interface{}{
			"rule": "702.117a",
		},
	})
	return &CostPaymentResult{}, nil
}

// ---------------------------------------------------------------------------
// Stack / per-turn predicates
// ---------------------------------------------------------------------------

// IsSurgeCast reports whether a StackItem was cast for its surge cost.
func IsSurgeCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["surge_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellSurgedThisTurn returns true if any spell was cast for its surge
// cost by `seatIdx` during the current turn.
func SpellSurgedThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_surged_this_turn:"+itoa(seatIdx)] > 0
}
