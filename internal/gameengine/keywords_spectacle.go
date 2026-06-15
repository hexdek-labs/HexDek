package gameengine

// keywords_spectacle.go — CR §702.137 Spectacle cast helper (Rivals of
// Ixalan / Ravnica Allegiance).
//
// Spectacle is a §601.2f alternative cost: "You may cast this spell for
// its spectacle cost rather than its mana cost if an opponent lost life
// this turn." The keyword surface (HasSpectacle, SpectacleCost,
// CanPaySpectacle) already lives in keywords_combat.go; this file adds
// the cast entry point, mirroring the shape of CastFlashback /
// CastBuyback / CastWarp.

// CastWithSpectacle casts `card` from `seatIdx`'s hand for its spectacle
// alt cost. CR §702.137a + §601.2f.
//
// Preconditions:
//   - card carries the spectacle keyword (HasSpectacle).
//   - At least one of `seatIdx`'s living opponents has lost life this
//     turn (CanPaySpectacle). Per §702.137a this is the spectacle
//     condition; any source of life loss qualifies (combat damage,
//     non-combat damage, paid life-loss costs the opponent took
//     voluntarily, drain effects, etc.) because Turn.LifeLost tracks
//     all of those.
//   - card is in `seatIdx`'s hand.
//   - seat can afford `spectacleCost` mana. Pass -1 to fall back to the
//     printed spectacle cost via SpectacleCost(card).
//
// On success:
//   - The spectacle cost is paid out of the seat's mana pool.
//   - The card is removed from hand.
//   - A StackItem is pushed with CostMeta:
//       "spectacle_cast"  = true
//       "alt_cost"        = "spectacle"
//       "spectacle_cost"  = <amount paid>
//     so the cast trail surfaces the alt cost for analytics, hat
//     introspection, and any future resolve-time hooks.
//   - The seat-level flag "spell_spectacle_this_turn:<seat>" is set —
//     mirrors the flashback / warp markers for cards that key off "if a
//     spectacle spell was cast this turn." Cleared in cleanup with the
//     other per-turn flags.
//   - A "spectacle_cast" event is logged with rule 702.137a.
//
// Returns a minimal CostPaymentResult on success, or a CastError naming
// the failure mode (no_spectacle_keyword, spectacle_condition_unmet,
// invalid_spectacle_cost, insufficient_mana, not_in_hand). Mirrors
// CastFlashback's intentionally narrow surface — callers needing the
// full cast pipeline (cast triggers, storm count, priority round)
// should go through CastFromZone with a permission instead.
func CastWithSpectacle(gs *GameState, seatIdx int, card *Card, spectacleCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasSpectacle(card) {
		return nil, &CastError{Reason: "no_spectacle_keyword"}
	}
	if !CanPaySpectacle(gs, seatIdx) {
		return nil, &CastError{Reason: "spectacle_condition_unmet"}
	}

	// Default to the printed spectacle cost when caller passes -1.
	if spectacleCost < 0 {
		spectacleCost = SpectacleCost(card)
	}
	if spectacleCost < 0 {
		return nil, &CastError{Reason: "invalid_spectacle_cost"}
	}

	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the spectacle alternative
	// cost too (cast from hand).
	spectacleCost = EffectiveAlternativeCost(gs, card, seatIdx, spectacleCost, CastContext{})
	if seat.ManaPool < spectacleCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// Remove from hand — fail before paying so we don't leak mana when
	// the caller hands us a card not actually in `seatIdx`'s hand.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}

	// Pay the spectacle cost.
	seat.ManaPool -= spectacleCost
	SyncManaAfterSpend(seat)
	if spectacleCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: spectacleCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "spectacle_cast",
				"keyword": "spectacle",
				"rule":    "601.2f",
			},
		})
	}

	// Push onto the stack flagged as a spectacle cast.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"spectacle_cast": true,
			"alt_cost":       "spectacle",
			"spectacle_cost": spectacleCost,
		},
	}
	PushStackItem(gs, item)

	// Mark the seat as having cast a spectacle spell this turn — used by
	// any card that asks "if a spectacle spell was cast this turn?".
	// Cleared in the cleanup step alongside other "this turn" flags.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_spectacle_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "spectacle_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: spectacleCost,
		Details: map[string]interface{}{
			"rule": "702.137a",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellSpectacleThisTurn reports whether seatIdx cast at least one
// spectacle spell during the current turn. Mirrors SpellWarpedThisTurn /
// the flashback equivalent for cards that key off the trail.
func SpellSpectacleThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_spectacle_this_turn:"+itoa(seatIdx)] > 0
}
