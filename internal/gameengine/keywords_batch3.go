package gameengine

// ============================================================================
// keywords_batch3.go — Alternative casting cost keywords (Batch 3)
//
// Implements: Emerge, Replicate, Prototype, Casualty, Squad.
// Also provides requested API wrappers for keywords already implemented
// under different function names in other files.
// ============================================================================

// ---------------------------------------------------------------------------
// Emerge — CR §702.119
// "You may cast this spell by sacrificing a creature and paying the emerge
// cost reduced by that creature's mana value."
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Offering — CR §702.48  (wrapper for existing OfferingReduction)
// "You may cast this as though it had flash by sacrificing a [type].
// The total cost is reduced by the sacrificed permanent's mana cost."
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Splice — CR §702.47  (wrappers for existing ApplySplice/HasSplice)
// "As you cast an Arcane spell, you may reveal this card from your hand
// and pay its splice cost. If you do, add this card's text to that spell."
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Replicate — CR §702.56
// "When you cast this spell, copy it for each time you paid its replicate
// cost. You may choose new targets for the copies."
// ---------------------------------------------------------------------------

// HasReplicate returns true if the card has the replicate keyword.
func HasReplicate(card *Card) bool {
	return cardHasKeywordByName(card, "replicate")
}

// ReplicateCost returns the replicate cost from keyword args. Returns 0
// when the parser didn't capture a cost — the cast gate already requires
// a positive cost before offering replicate, so unknown reads as decline.
func ReplicateCost(card *Card) int {
	c, ok := keywordArgCostStrict(card, "replicate")
	if !ok {
		return 0
	}
	return c
}

// ApplyReplicate pays the replicate cost `copies` times and puts that
// many copies of the spell onto the stack above the original. Per CR
// §702.56a / §707.10, each copy has the same characteristics as the
// spell, the controller of each copy is the player who put it on the
// stack, and the copies use the same targets as the original (the
// controller may choose new targets via a separate retarget step).
//
// Returns the number of copies actually placed on the stack. Returns 0
// (and logs a replicate_fail event) if the controller cannot pay the
// total replicate cost — replicate is a one-shot decision made when the
// spell is cast, so partial payment is not permitted.
func ApplyReplicate(gs *GameState, item *StackItem, copies int) int {
	if gs == nil || item == nil || item.Card == nil || copies <= 0 {
		return 0
	}
	cost := ReplicateCost(item.Card)
	seatIdx := item.Controller
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return 0
	}
	seat := gs.Seats[seatIdx]

	totalCost := cost * copies
	if seat.ManaPool < totalCost {
		gs.LogEvent(Event{
			Kind:   "replicate_fail",
			Seat:   seatIdx,
			Source: item.Card.DisplayName(),
			Amount: copies,
			Details: map[string]interface{}{
				"cost_each":  cost,
				"total_cost": totalCost,
				"available":  seat.ManaPool,
				"rule":       "702.56",
			},
		})
		return 0
	}
	seat.ManaPool -= totalCost
	SyncManaAfterSpend(seat)

	gs.LogEvent(Event{
		Kind:   "replicate_pay",
		Seat:   seatIdx,
		Source: item.Card.DisplayName(),
		Amount: copies,
		Details: map[string]interface{}{
			"cost_each":  cost,
			"total_cost": totalCost,
			"rule":       "702.56",
		},
	})

	// Per CR §707.10, a copy of a spell has the same characteristics
	// (name, mana cost, types, colors, P/T, text) as the spell being
	// copied — only the controller and (optionally) targets can differ.
	madeCopies := make([]*Card, 0, copies)
	for i := 0; i < copies; i++ {
		copyCard := &Card{
			Name:          item.Card.Name,
			Owner:         item.Card.Owner,
			BasePower:     item.Card.BasePower,
			BaseToughness: item.Card.BaseToughness,
			Types:         append([]string(nil), item.Card.Types...),
			Colors:        append([]string(nil), item.Card.Colors...),
			CMC:           item.Card.CMC,
			AST:           item.Card.AST,
		}
		MintCopyInstanceID(gs, copyCard, item.Card.InstanceID, currentMintEnablerID(gs))
		copyItem := &StackItem{
			Controller: seatIdx, // CR §707.10 — controller is the player who put the copy on the stack.
			Card:       copyCard,
			Effect:     item.Effect,
			Targets:    append([]Target(nil), item.Targets...), // §707.10c — same targets unless retargeted.
			Kind:       item.Kind,
			IsCopy:     true, // CR §707.10
		}
		copyItem.ID = nextStackID(gs)
		gs.Stack = append(gs.Stack, copyItem)
		madeCopies = append(madeCopies, copyCard)

		gs.LogEvent(Event{
			Kind:   "replicate_copy",
			Seat:   seatIdx,
			Source: copyCard.DisplayName(),
			Details: map[string]interface{}{
				"stack_id":   copyItem.ID,
				"stack_size": len(gs.Stack),
				"copy_index": i + 1,
				"rule":       "702.56+706.10",
			},
		})
	}
	// CR §702.137a / "whenever you copy a spell" — each replicate copy fires
	// the canonical copy-trigger fan-out (magecraft + spell_copied). Fired
	// after the push loop so the copies stay contiguous on the stack (mirrors
	// ApplyStormCopy) and the resulting triggers land above them.
	for _, cc := range madeCopies {
		FireSpellCopyTriggers(gs, seatIdx, cc, item.Card)
	}
	return copies
}

// ---------------------------------------------------------------------------
// Retrace — CR §702.81  (wrappers for existing CanCastRetrace)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Jump-Start — CR §702.133  (wrappers for existing HasJumpStart)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Entwine — CR §702.42  (wrappers for existing CanPayEntwine)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Surge — CR §702.117  (wrapper for existing CanPaySurge)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Undaunted — CR §702.125  (already implemented; no new wrapper needed)
// UndauntedReduction is in keywords_combat.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Assist — CR §702.132  (already implemented; no new wrapper needed)
// AssistReduction is in keywords_combat.go.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Miracle — CR §702.94  (wrapper for existing CanCastMiracle)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Prototype — CR §702.160
// "You may cast this spell with different mana cost, color, power, and
// toughness. It keeps its abilities and other characteristics."
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Bargain — CR §702.166
// Already implemented in costs.go as BargainAdditionalCost().
// No additional functions needed.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Casualty — CR §702.153
// "As an additional cost to cast this spell, you may sacrifice a creature
// with power N or greater. When you do, copy this spell."
// ---------------------------------------------------------------------------

// HasCasualty returns true if the card has the casualty keyword.
func HasCasualty(card *Card) bool {
	return cardHasKeywordByName(card, "casualty")
}

// CasualtyMinPower returns the minimum power required for the sacrificed
// creature from the keyword args. Returns 0 when the parser didn't
// capture the printed N — the cast gate treats 0 as "decline casualty"
// (pre-r61.1 the CMC fallback demanded a power-4 sacrifice for
// "casualty 1" on a 4-CMC spell: wrong cost, wrong direction).
func CasualtyMinPower(card *Card) int {
	c, ok := keywordArgCostStrict(card, "casualty")
	if !ok {
		return 0
	}
	return c
}

// PayCasualty sacrifices a creature with power >= minPower as an additional
// cost for casualty. Returns true if a creature was sacrificed.
func PayCasualty(gs *GameState, seatIdx int, minPower int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	seat := gs.Seats[seatIdx]

	// Find the cheapest creature (lowest CMC) with sufficient power.
	var best *Permanent
	bestCMC := 999
	for _, p := range seat.Battlefield {
		if p == nil || p.Card == nil || !p.IsCreature() {
			continue
		}
		if p.Power() < minPower {
			continue
		}
		if p.Card.CMC < bestCMC {
			best = p
			bestCMC = p.Card.CMC
		}
	}
	if best == nil {
		return false
	}

	name := best.Card.DisplayName()
	SacrificePermanent(gs, best, "casualty")

	gs.LogEvent(Event{
		Kind:   "casualty",
		Seat:   seatIdx,
		Source: name,
		Amount: minPower,
		Details: map[string]interface{}{
			"min_power": minPower,
			"sac_power": best.Power(),
			"rule":      "702.153",
		},
	})
	return true
}

// CanPayCasualty checks if the player has a creature with sufficient power
// to sacrifice for casualty.
func CanPayCasualty(gs *GameState, seatIdx int, minPower int) bool {
	if gs == nil || seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return false
	}
	for _, p := range gs.Seats[seatIdx].Battlefield {
		if p == nil || !p.IsCreature() {
			continue
		}
		if p.Power() >= minPower {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Squad — CR §702.157
// "As an additional cost to cast this spell, you may pay {cost} any number
// of times. When this creature enters the battlefield, create a token
// that's a copy of it for each time the squad cost was paid."
// ---------------------------------------------------------------------------
