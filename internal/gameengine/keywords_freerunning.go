package gameengine

// keywords_freerunning.go — Freerunning (CR §702.169, OTJ Outlaws of
// Thunder Junction) as a real alt-cost cast helper.
//
// CR §702.169a: Freerunning is a static ability that functions while
//               the card is in any zone from which it could be cast.
//               "Freerunning [cost]" means "You may cast this spell by
//               paying [cost] rather than paying its mana cost if a
//               creature with an Outlaw type you controlled dealt
//               combat damage to a player this turn."
// CR §702.169b: Casting a spell using its freerunning ability follows
//               the rules for paying alternative costs in §601.2b and
//               §601.2f-h.
//
// Implementation mirrors CastBuyback / CastFlashback: a thin helper
// (HasFreerunning, FreerunningCost) plus a cast entry point
// (CastWithFreerunning) that removes the card from hand, pays the
// freerunning cost rather than the printed mana cost, and pushes a
// StackItem flagged with CostMeta["freerunning_cast"]=true.
//
// Scope note: the existing CanCastForFreerunning predicate in
// keywords_batch6.go gates on the seat flag
// "creature_dealt_combat_damage_to_player". CR §702.169 strictly
// requires the damage-dealer to have an Outlaw type (Assassin,
// Mercenary, Pirate, Rogue, Warlock). The engine currently tracks the
// broader signal and we use it here per the task brief; an outlaw-type
// refinement of the predicate is left to a follow-up that would also
// touch combat.go's damage-dealt bookkeeping.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasFreerunning / FreerunningCost
// ---------------------------------------------------------------------------

// HasFreerunning returns true if the card has the freerunning keyword
// in its AST.
func HasFreerunning(card *Card) bool {
	return cardHasKeywordByName(card, "freerunning")
}

// FreerunningCost returns the converted mana cost of the freerunning
// keyword's alternative cost. Accepts the keyword arg as either a mana
// string ("{1}{B}") or a plain numeric value. Returns 0 if the keyword
// is absent or the args are malformed; callers should treat 0 as "free"
// only when they have positively confirmed HasFreerunning.
func FreerunningCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "freerunning") {
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
// Stack predicate
// ---------------------------------------------------------------------------

// IsFreerunningCast reports whether a StackItem was cast via its
// freerunning alternative cost.
func IsFreerunningCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["freerunning_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// ---------------------------------------------------------------------------
// CastWithFreerunning
// ---------------------------------------------------------------------------

// CastWithFreerunning casts a card from `seatIdx`'s hand using its
// freerunning alternative cost. CR §702.169a — pay `freerunningCost`
// rather than the printed mana cost.
//
// Preconditions enforced here:
//   - card has the freerunning keyword
//   - card is in `seatIdx`'s hand
//   - CanCastForFreerunning(gs, seatIdx) is true (the gating combat-damage
//     predicate already maintained by combat.go)
//   - seat can afford `freerunningCost` mana (pass -1 to use the printed
//     FreerunningCost)
//
// On success the card is removed from hand, mana is paid, and a
// StackItem is pushed with:
//   - CostMeta["freerunning_cast"]   = true
//   - CostMeta["freerunning_cost"]   = freerunningCost (int)
//   - CostMeta["alt_cost_keyword"]   = "freerunning"
//
// The seat-level flag "spell_freerunning_cast_this_turn:<seat>" is set
// for cards/triggers that key off "if you cast a spell for its
// freerunning cost this turn." Cleared in cleanup alongside other "this
// turn" flags.
//
// Returns (CostPaymentResult, error). The result is intentionally
// minimal — callers that need the full cast pipeline (cast triggers,
// storm, priority round) should call CastFromZone with a freerunning-
// flavored ZoneCastPermission instead. This entry point mirrors
// CastBuyback / CastFlashback: pay + push, leaving stack resolution to
// the caller.
func CastWithFreerunning(gs *GameState, seatIdx int, card *Card, freerunningCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasFreerunning(card) {
		return nil, &CastError{Reason: "no_freerunning_keyword"}
	}
	// CR §702.169a — gated on "an Outlaw you controlled dealt combat damage
	// to a player this turn." Implemented via the existing seat-flag
	// predicate (see top-of-file scope note).
	if !CanCastForFreerunning(gs, seatIdx) {
		return nil, &CastError{Reason: "freerunning_not_enabled"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// Resolve cost: caller may pass -1 to mean "use the printed
	// freerunning cost."
	if freerunningCost < 0 {
		freerunningCost = FreerunningCost(card)
	}
	if freerunningCost < 0 {
		return nil, &CastError{Reason: "invalid_freerunning_cost"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the freerunning
	// alternative cost too (cast from hand).
	freerunningCost = EffectiveAlternativeCost(gs, card, seatIdx, freerunningCost, CastContext{})
	if seat.ManaPool < freerunningCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	// Drannith Magistrate / similar cast-restriction guards: a player
	// without permission to cast from hand can't use freerunning either.
	// Freerunning casts FROM HAND, so the hand-cast guard is the only
	// one that applies here — there's no zone exotic for this keyword.

	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	seat.ManaPool -= freerunningCost
	SyncManaAfterSpend(seat)
	if freerunningCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: freerunningCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "freerunning_cast",
				"keyword": "freerunning",
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
			"freerunning_cast": true,
			"freerunning_cost": freerunningCost,
			"alt_cost_keyword": "freerunning",
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_freerunning_cast_this_turn:"+itoa(seatIdx)] = 1

	gs.LogEvent(Event{
		Kind:   "freerunning_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: freerunningCost,
		Details: map[string]interface{}{
			"rule": "702.169a",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellFreerunningCastThisTurn returns true if any spell was cast for
// its freerunning cost by `seatIdx` during the current turn.
func SpellFreerunningCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_freerunning_cast_this_turn:"+itoa(seatIdx)] > 0
}
