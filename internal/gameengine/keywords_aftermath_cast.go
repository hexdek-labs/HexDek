package gameengine

// keywords_aftermath_cast.go — Aftermath cast helper (CR §702.128,
// Amonkhet 2017).
//
// CR §702.128a: Aftermath is an ability word found on the back half
//               of certain split cards. "Aftermath" means "Cast this
//               half of this split card only from your graveyard."
// CR §702.128b: If a spell with aftermath would be put into a
//               graveyard from anywhere, exile it instead. This
//               replacement applies whether the aftermath half was
//               cast or not — the exile-on-resolve routing is the
//               common case for a successful cast.
//
// Existing surface (pre-this-file):
//   - HasAftermath(card)         in keywords_combat.go — keyword detector
//   - CanCastAftermath(gs, seat, card)
//                                in keywords_batch5.go — gating predicate
//                                (card in graveyard + has aftermath keyword)
//   - NewAftermathCastPermission in keywords_combat.go — ZoneCastPermission
//                                factory used by the cost-pipeline path
//
// What this file adds: CastWithAftermath, a thin entry point that
// mirrors CastFlashback / CastMayhem / CastFreerunning — it pays the
// printed mana cost, removes the card from the graveyard, and pushes
// a StackItem flagged for resolve-time exile (CR §702.128b). The
// "alt-cost" framing applies even though aftermath is technically a
// PERMISSION rather than an alternative cost: the player pays the
// card's printed mana cost, which is the only legal way to cast the
// back half. We name the helper "alt-path" rather than "alt-cost"
// in events to keep the rules semantics accurate.
//
// Sorcery-speed enforcement: aftermath cards (the back halves) are
// always sorceries (CR §702.128a — "this half" refers to the printed
// type of the back face, which is always sorcery). CastWithAftermath
// enforces sorcery-speed timing via isSorceryTiming so callers don't
// silently slip an instant-speed cast through.

// CastWithAftermath casts the aftermath back half of a split card
// from `seatIdx`'s graveyard. CR §702.128a — pay the printed mana
// cost; the aftermath ability is the permission. Resolution routes
// the card to exile per §702.128b.
//
// Preconditions enforced here:
//   - card has the aftermath keyword
//   - card is in seatIdx's graveyard
//   - it is sorcery-speed timing for seatIdx (active player, main
//     phase, empty stack — same gate isSorceryTiming applies elsewhere)
//   - seat can afford the printed mana cost (or the explicit cost
//     passed in; pass -1 to use the card's printed CMC)
//
// On success:
//   - card is removed from graveyard
//   - mana is paid
//   - a StackItem is pushed with CostMeta:
//       aftermath_cast    = true
//       exile_on_resolve  = true   (CR §702.128b — canonical handoff
//                                   to ShouldExileOnResolve in stack.go)
//       zone_cast_keyword = "aftermath"
//       aftermath_cost    = int (the amount actually paid)
//   - "aftermath_cast" event logged
//   - "spell_aftermath_cast_this_turn:<seat>" flag set for any future
//     trigger that keys off "if you cast a card with aftermath this
//     turn." Cleared with other this-turn flags.
//
// Returns (CostPaymentResult, error). Mirrors CastFlashback /
// CastMayhem / CastFreerunning shape: pay + push, leaving stack
// resolution to the caller.
func CastWithAftermath(gs *GameState, seatIdx int, card *Card, manaCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasAftermath(card) {
		return nil, &CastError{Reason: "no_aftermath_keyword"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}

	// Sorcery-speed gate. CR §702.128a — the aftermath half is a
	// sorcery, so the standard sorcery-speed timing restriction
	// applies. Enforced here (defense in depth) even though the
	// upstream cast pipeline also enforces it, because
	// CastWithAftermath is a direct entry point and tests + ad-hoc
	// callers shouldn't need to know whether the outer pipeline ran.
	if !isSorceryTiming(gs, seatIdx) {
		return nil, &CastError{Reason: "sorcery_speed_only"}
	}

	// Resolve cost: pass -1 to mean "use the printed mana cost."
	if manaCost < 0 {
		manaCost = manaCostOf(card)
	}
	if manaCost < 0 {
		return nil, &CastError{Reason: "invalid_aftermath_cost"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the aftermath alternative
	// cost too (the aftermath half is cast from the graveyard).
	manaCost = EffectiveAlternativeCost(gs, card, seatIdx, manaCost, CastContext{FromGraveyard: true})
	if seat.ManaPool < manaCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// Drannith Magistrate: opponents can't cast from non-hand zones.
	if drannithRestrictsZoneCast(gs, seatIdx) {
		gs.LogEvent(Event{
			Kind:   "cast_suppressed",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason": "drannith_magistrate",
				"zone":   ZoneGraveyard,
				"rule":   "601.2a",
			},
		})
		return nil, &CastError{Reason: "drannith_magistrate"}
	}

	// Remove from graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil, &CastError{Reason: "not_in_graveyard"}
	}

	// Pay mana.
	seat.ManaPool -= manaCost
	SyncManaAfterSpend(seat)
	if manaCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: manaCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "aftermath_cast",
				"keyword": "aftermath",
				"rule":    "601.2f",
			},
		})
	}

	// Push the stack item with the exile-on-resolve hand-off.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"aftermath_cast":    true,
			"exile_on_resolve":  true,
			"zone_cast_keyword": "aftermath",
			"aftermath_cost":    manaCost,
		},
	}
	PushStackItem(gs, item)

	// Seat-level marker for "you cast a card with aftermath this turn"
	// triggers, cleared with other this-turn flags.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_aftermath_cast_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "aftermath_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: manaCost,
		Details: map[string]interface{}{
			"rule": "702.128a",
		},
	})

	return &CostPaymentResult{}, nil
}

// IsAftermathCast reports whether a StackItem was put on the stack
// via the aftermath alt-path. Mirrors IsBoughtBack / IsFreerunningCast.
func IsAftermathCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["aftermath_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// SpellAftermathCastThisTurn returns true if any spell was cast via
// its aftermath ability by `seatIdx` during the current turn.
func SpellAftermathCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_aftermath_cast_this_turn:"+itoa(seatIdx)] > 0
}
