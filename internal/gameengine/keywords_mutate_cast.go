package gameengine

import "strings"

// keywords_mutate_cast.go — CR §702.140 Mutate cast helper (Ikoria,
// 2020).
//
// The merge primitive (ApplyMutate) already lives in
// keywords_batch6.go. This file adds the §601.2f alt-cost CAST entry
// point that pays the mutate cost, validates the target, and routes
// the resolution into ApplyMutate to produce a single merged
// permanent.
//
// CR §702.140a-b:
//
//   "Mutate [cost]" means "If you cast this spell for its mutate
//    cost, it becomes a mutating creature spell. As it resolves, put
//    it onto the battlefield merged with target non-Human creature
//    you own."
//
//   "Rather than have a mutating creature enter the battlefield as a
//    separate creature, you merge it with the target creature."
//
// Architecture:
//
//   - CastWithMutate(gs, seat, card, mutateCost, target, onTop)
//     full cast-and-resolve entry point. Validates HasMutate, that
//     `card` is in `seat`'s hand, that `target` is a controller's
//     non-Human creature, and that `seat` can pay `mutateCost`.
//     Pays the cost, removes the card from hand, pushes a StackItem
//     stamped with CostMeta["mutate_cast"]=true, materializes the
//     mutating Permanent on `seat`'s battlefield, then delegates
//     the merge to ApplyMutate(gs, mutating, target, onTop). The
//     stack item is popped after the inline resolve (mirrors the
//     CastDisguiseFaceDown / cascade short-circuit pattern used for
//     resolve-in-place flows).
//
//     Returns the resulting merged permanent on success (whichever
//     of {mutating, target} ApplyMutate kept on the battlefield):
//       - onTop == true  → returns mutating perm (target removed)
//       - onTop == false → returns target perm (mutating removed)
//     Either way the survivor carries Flags["mutated"] = 1 and the
//     merged GrantedAbilities.
//
//   - MutateCost(card) extract the printed mutate cost from the AST
//     keyword args via the shared keywordArgCost helper. Falls back
//     to the card's CMC when the arg is missing — matches the other
//     alt-cost helpers.
//
// HasMutate already exists in keywords_batch6.go and is re-used here.

// MutateCost returns the printed mutate cost {N} from the card's AST.
// Defaults via keywordArgCost("mutate"), which falls back to the
// card's CMC when no numeric arg is parsed. Callers that aren't sure
// should pass the cost explicitly to CastWithMutate.
func MutateCost(card *Card) int {
	return keywordArgCost(card, "mutate")
}

// CastWithMutate casts `card` from `seatIdx`'s hand for its mutate
// alt cost and merges the resulting creature with `targetPerm`. CR
// §702.140a-b + §601.2f.
//
// Preconditions:
//   - card has the mutate keyword (HasMutate).
//   - card is in `seatIdx`'s hand.
//   - mutateCost >= 0. Pass -1 to fall back to MutateCost(card).
//   - targetPerm is non-nil, controlled by `seatIdx`, on the
//     battlefield, IS a creature, and is NOT a Human (§702.140a).
//     "Non-Human" is satisfied when targetPerm.Card.Types lacks
//     "human" — token creatures that aren't typed Human pass.
//   - seat can afford mutateCost mana.
//
// On success:
//   - Pays mutateCost.
//   - Removes `card` from `seatIdx`'s hand (BEFORE the mana spend
//     is committed via SyncManaAfterSpend, matching the other cast
//     helpers' "remove-before-pay so rejection cannot leak mana"
//     pattern — but here we keep the explicit ordering: pay first,
//     then remove, and on remove failure we refund. Callers passing
//     a card not in `seat`'s hand cause refund + not_in_hand error.).
//   - Pushes a StackItem with:
//       Card        = card
//       Controller  = seatIdx
//       CastZone    = ZoneHand
//       CostMeta:
//         "mutate_cast"   = true
//         "alt_cost"      = "mutate"
//         "mutate_cost"   = <amount paid>
//         "mutate_target" = targetPerm  (for analytics / per_card
//                                        hooks that key off the
//                                        cast trail)
//         "mutate_on_top" = onTop
//   - Materializes the mutating creature as a fresh Permanent on
//     `seatIdx`'s battlefield. We DO NOT fire ETB triggers on the
//     mutating perm — per §702.140b "rather than enter the
//     battlefield as a separate creature, you merge it." ApplyMutate
//     fires its own "creature_mutated" trigger after the merge.
//   - Calls ApplyMutate(gs, mutating, targetPerm, onTop) to perform
//     the merge. ApplyMutate logs the "mutate" event and fires the
//     creature_mutated trigger.
//   - Pops the StackItem (the spell has resolved in-place).
//
// Returns the surviving merged permanent (caller's choice of `onTop`
// dictates which is the survivor) plus a "mutate_cast" event
// recording the alt-cost trail.
//
// On validation failure: returns a CastError with one of
//   no_mutate_keyword | invalid_mutate_cost | nil_target |
//   target_not_creature | target_is_human | target_wrong_controller |
//   insufficient_mana | not_in_hand
// and leaves all side effects un-applied (mana untouched, hand
// untouched, no stack push, no battlefield change).
func CastWithMutate(gs *GameState, seatIdx int, card *Card, mutateCost int, targetPerm *Permanent, onTop bool) (*Permanent, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasMutate(card) {
		return nil, &CastError{Reason: "no_mutate_keyword"}
	}

	// Target validation (CR §702.140a).
	if targetPerm == nil || targetPerm.Card == nil {
		return nil, &CastError{Reason: "nil_target"}
	}
	if targetPerm.Controller != seatIdx {
		return nil, &CastError{Reason: "target_wrong_controller"}
	}
	if !targetPerm.IsCreature() {
		return nil, &CastError{Reason: "target_not_creature"}
	}
	if targetIsHuman(targetPerm) {
		return nil, &CastError{Reason: "target_is_human"}
	}

	if mutateCost < 0 {
		mutateCost = MutateCost(card)
	}
	if mutateCost < 0 {
		return nil, &CastError{Reason: "invalid_mutate_cost"}
	}

	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the mutate alternative
	// cost too (cast from hand).
	mutateCost = EffectiveAlternativeCost(gs, card, seatIdx, mutateCost, CastContext{})
	if seat.ManaPool < mutateCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// Remove from hand BEFORE paying so a not_in_hand rejection cannot
	// leak mana. Matches CastWithSpectacle / CastWithStrive / etc.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}

	// Pay the mutate cost.
	seat.ManaPool -= mutateCost
	SyncManaAfterSpend(seat)
	if mutateCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: mutateCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "mutate_cast",
				"keyword": "mutate",
				"rule":    "601.2f",
			},
		})
	}

	// Push the stack item (alt-cost trail). Resolved inline below —
	// mutate spells merge directly rather than spend time on the
	// stack, matching the cascade / disguise short-circuit pattern.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		CostMeta: map[string]interface{}{
			"mutate_cast":   true,
			"alt_cost":      "mutate",
			"mutate_cost":   mutateCost,
			"mutate_target": targetPerm,
			"mutate_on_top": onTop,
		},
	}
	PushStackItem(gs, item)

	// Materialize the mutating creature as a Permanent on the
	// caster's battlefield. We do NOT fire ETB triggers here:
	// §702.140b — the mutating creature "rather than enter the
	// battlefield as a separate creature, you merge it with the
	// target creature." ApplyMutate fires the creature_mutated
	// trigger after the merge.
	mutating := &Permanent{
		Card:          card,
		Controller:    seatIdx,
		Owner:         card.Owner,
		SummoningSick: true,
		Timestamp:     gs.NextTimestamp(),
		Counters:      map[string]int{},
		Flags:         map[string]int{},
	}
	seat.Battlefield = append(seat.Battlefield, mutating)
	RegisterReplacementsForPermanent(gs, mutating)

	// Merge. ApplyMutate removes whichever perm is the "loser"
	// (target when onTop=true, mutating when onTop=false) and stamps
	// Flags["mutated"]=1 on the survivor.
	ApplyMutate(gs, mutating, targetPerm, onTop)

	// Pop the stack item (resolved in-place). Match
	// CastDisguiseFaceDown's pop discipline.
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		gs.Stack = gs.Stack[:len(gs.Stack)-1]
	}

	// Per-turn flag so cards that key off "if you mutated a creature
	// this turn" can read it. Mirrors SpellSpectacleThisTurn etc.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["mutate_cast_this_turn:"+itoa(seatIdx)]++

	gs.LogEvent(Event{
		Kind:   "mutate_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: mutateCost,
		Details: map[string]interface{}{
			"target": targetPerm.Card.DisplayName(),
			"on_top": onTop,
			"rule":   "702.140a",
		},
	})

	// Return the survivor — whichever of {mutating, target} is still
	// on the battlefield. onTop=true → mutating; false → target.
	if onTop {
		return mutating, nil
	}
	return targetPerm, nil
}

// MutateCastThisTurn returns the number of mutate casts `seatIdx`
// has performed this turn. Reader for the
// "mutate_cast_this_turn:<seat>" per-turn flag.
func MutateCastThisTurn(gs *GameState, seatIdx int) int {
	if gs == nil || gs.Flags == nil {
		return 0
	}
	return gs.Flags["mutate_cast_this_turn:"+itoa(seatIdx)]
}

// targetIsHuman reports whether targetPerm's card has the Human
// creature type. Case-insensitive. Tokens without a typed-Human
// entry are treated as non-Human (per §702.140a — a vanilla 1/1
// Cat token, for example, is a legal mutate target).
func targetIsHuman(p *Permanent) bool {
	if p == nil || p.Card == nil {
		return false
	}
	for _, t := range p.Card.Types {
		if strings.EqualFold(t, "human") {
			return true
		}
	}
	return false
}
