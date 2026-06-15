package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

// keywords_cleave.go — CR §702.158 Cleave (Innistrad: Crimson Vow, 2021).
//
// Cleave is a §601.2f ALTERNATIVE cost: "Cleave {cost}: you may pay
// {cost}. If you do, remove the text in square brackets." Per §702.158a
// the spell still goes on the stack like a normal cast, but its
// printed text has TWO variants:
//
//   - Default (cleave NOT paid): the bracketed text is included.
//   - Cleave  (cleave paid):     the bracketed text is removed.
//
// Concretely, "Dig Up the Body" (the canonical example) reads:
//
//   "Return target [non-Zombie] creature card from your graveyard to
//    your hand."
//
// Cleave-cast, it instead reads "Return target creature card..." (the
// "non-Zombie" restriction is removed). The two variants are
// semantically distinct ASTs / effects; HexDek represents this by
// keeping BOTH effects on the card's AST as separate Activated
// abilities (the resolver code below picks between them).
//
// Architecture:
//
//   - HasCleave(card)            keyword detection.
//   - CleaveCost(card)           extract {N} from the keyword args.
//   - CleaveEffect(card)         returns the brackets-removed variant
//                                from the card's AST.
//   - BaseSpellEffect(card)      returns the default (bracketed)
//                                variant. Thin alias of
//                                collectSpellEffect for symmetry.
//   - CastWithCleave             alt-cost cast entry point. Pays the
//                                cleave cost, swaps in CleaveEffect on
//                                the StackItem, sets
//                                CleaveActive=true, stamps the
//                                cleave_cast CostMeta tags.
//
// AST convention for the two variants: the card's AST.Abilities holds
// the base (bracketed) effect as its FIRST Activated/Static ability
// (so collectSpellEffect picks it up unchanged for non-cleave casts)
// and the cleave (brackets-removed) effect as a SECOND Activated
// ability. CleaveEffect walks past the first effect-carrying Activated
// and returns the second one. Cards without a second Activated have no
// cleave variant available — CleaveEffect returns nil and
// CastWithCleave rejects with no_cleave_variant.

// HasCleave reports whether the card has the cleave keyword.
func HasCleave(card *Card) bool {
	return cardHasKeywordByName(card, "cleave")
}

// CleaveCost returns the printed cleave cost {N}. Defaults to
// keywordArgCost("cleave") which itself falls back to the card's CMC
// when no numeric arg is parsed — that fallback is wrong for cleave
// (the printed cleave cost is usually CHEAPER than the spell's CMC,
// not equal), so callers that aren't sure should pass the cost
// explicitly to CastWithCleave.
func CleaveCost(card *Card) int {
	return keywordArgCost(card, "cleave")
}

// BaseSpellEffect returns the default (bracketed) variant of the
// spell's effect. Thin symmetry alias around collectSpellEffect, named
// to read naturally next to CleaveEffect at the cleave-cast site.
func BaseSpellEffect(card *Card) gameast.Effect {
	return collectSpellEffect(card)
}

// CleaveEffect returns the brackets-removed variant of the spell's
// effect, taken from the second effect-carrying Activated ability on
// the card's AST. Returns nil if no second Activated effect is
// present — caller must treat this as "this card has no cleave
// variant to cast."
//
// The walk specifically counts Activated abilities (matching what
// collectSpellEffect picks up), so static keyword nodes interleaved
// with the effect abilities don't shift the indexing.
func CleaveEffect(card *Card) gameast.Effect {
	if card == nil || card.AST == nil {
		return nil
	}
	seen := 0
	for _, ab := range card.AST.Abilities {
		a, ok := ab.(*gameast.Activated)
		if !ok || a.Effect == nil {
			continue
		}
		seen++
		if seen == 2 {
			return a.Effect
		}
	}
	return nil
}

// CastWithCleave casts `card` from `seatIdx`'s hand for its cleave
// alt cost. CR §702.158a + §601.2f.
//
// Preconditions:
//   - card carries the cleave keyword (HasCleave).
//   - card.AST exposes a second effect-carrying Activated ability that
//     CleaveEffect can return — without it the cleave cast is rejected
//     because there'd be no "brackets-removed" effect to resolve into.
//   - card is in `seatIdx`'s hand.
//   - seat can afford `cleaveCost` mana. Pass -1 to fall back to
//     CleaveCost(card).
//
// On success:
//   - The cleave cost is paid out of the seat's mana pool.
//   - The card is removed from hand (BEFORE the cleave-variant
//     lookup is committed so a not_in_hand rejection cannot leak
//     mana).
//   - A StackItem is pushed with:
//       Effect       = CleaveEffect(card)  — brackets-removed
//       CleaveActive = true
//       CastZone     = ZoneHand
//       CostMeta:
//         "cleave_cast" = true
//         "alt_cost"    = "cleave"
//         "cleave_cost" = <amount paid>
//   - The seat-level flag "spell_cleave_this_turn:<seat>" is set —
//     mirrors the flashback / spectacle markers for cards that key
//     off "if a cleave spell was cast this turn." Cleared in
//     cleanup with the other per-turn flags.
//   - A "cleave_cast" event is logged with rule 702.158a.
//
// Returns a minimal CostPaymentResult on success, or a CastError
// naming the failure mode (no_cleave_keyword, no_cleave_variant,
// insufficient_mana, invalid_cleave_cost, not_in_hand).
func CastWithCleave(gs *GameState, seatIdx int, card *Card, cleaveCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasCleave(card) {
		return nil, &CastError{Reason: "no_cleave_keyword"}
	}
	// Verify the brackets-removed variant exists. Doing this BEFORE
	// touching mana / hand keeps the failure clean.
	cleaveEff := CleaveEffect(card)
	if cleaveEff == nil {
		return nil, &CastError{Reason: "no_cleave_variant"}
	}

	if cleaveCost < 0 {
		cleaveCost = CleaveCost(card)
	}
	if cleaveCost < 0 {
		return nil, &CastError{Reason: "invalid_cleave_cost"}
	}

	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the cleave alternative
	// cost too (cast from hand).
	cleaveCost = EffectiveAlternativeCost(gs, card, seatIdx, cleaveCost, CastContext{})
	if seat.ManaPool < cleaveCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// Remove from hand BEFORE paying so a not_in_hand rejection cannot
	// leak mana. Matches CastWithSpectacle / CastWithStrive /
	// CastFlashback / CastWarp.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}

	// Pay the cleave cost.
	seat.ManaPool -= cleaveCost
	SyncManaAfterSpend(seat)
	if cleaveCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: cleaveCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "cleave_cast",
				"keyword": "cleave",
				"rule":    "601.2f",
			},
		})
	}

	item := &StackItem{
		Card:         card,
		Controller:   seatIdx,
		CastZone:     ZoneHand,
		Effect:       cleaveEff, // brackets-removed variant
		CleaveActive: true,
		CostMeta: map[string]interface{}{
			"cleave_cast": true,
			"alt_cost":    "cleave",
			"cleave_cost": cleaveCost,
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_cleave_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "cleave_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: cleaveCost,
		Details: map[string]interface{}{
			"rule": "702.158a",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellCleaveThisTurn reports whether seatIdx cast at least one cleave
// spell during the current turn. Mirrors SpellSpectacleThisTurn /
// SpellStriveThisTurn / SpellWarpedThisTurn.
func SpellCleaveThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_cleave_this_turn:"+itoa(seatIdx)] > 0
}
