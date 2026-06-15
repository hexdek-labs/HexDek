package gameengine

// keywords_mayhem.go — Mayhem (CR §702.187) as a real cast-from-graveyard
// alt-cost mechanic, gated on "you discarded this card this turn."
//
// CR §702.187a: Mayhem is a static ability that functions while the card
//               with mayhem is in a player's graveyard. "Mayhem [cost]"
//               means "You may cast this card from your graveyard for its
//               mayhem cost if you discarded it this turn."
// CR §702.187b: Casting a spell using its mayhem ability follows the rules
//               for paying alternative costs in §601.2b and §601.2f-h.
// CR §702.187c: If a spell with mayhem would be put into a graveyard from
//               the stack, exile it instead.
//
// Implementation mirrors keywords_flashback.go: a thin permission check
// (HasMayhem + MayhemDiscards[card] == gs.Turn), MayhemCost reads the
// keyword arg, and CastMayhem removes the card from its owner's graveyard,
// pays the mana, and pushes a StackItem with CostMeta["exile_on_resolve"]
// = true so the existing ResolveStackTop branch routes it to exile on
// resolution per §702.187c.
//
// The discard-side wiring lives in resolve.go's DiscardCard, which sets
// gs.MayhemDiscards[card] = gs.Turn whenever a card with mayhem hits a
// graveyard via the canonical discard path. The map is cleared in
// EndOfTurnCleanup so the "this turn" window closes correctly.

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasMayhem / MayhemCost
// ---------------------------------------------------------------------------

// HasMayhem returns true if the card has the mayhem keyword in its AST.
func HasMayhem(card *Card) bool {
	return cardHasKeywordByName(card, "mayhem")
}

// MayhemCost returns the converted mana cost of the mayhem keyword's
// alternative cost. Accepts the keyword arg as either a mana string
// ("{2}{R}") or a plain numeric value. Returns 0 if the keyword is
// absent or the args are malformed; callers should treat 0 as "free"
// only when they have positively confirmed HasMayhem.
func MayhemCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "mayhem") {
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
// MayhemEligible
// ---------------------------------------------------------------------------

// MayhemEligible returns true if `card` was discarded on the current turn
// and is therefore castable via its mayhem ability. CR §702.187a.
// Caller is still responsible for verifying that the card has the mayhem
// keyword (or a granted mayhem permission) and is in the right zone.
func MayhemEligible(gs *GameState, card *Card) bool {
	if gs == nil || card == nil || gs.MayhemDiscards == nil {
		return false
	}
	return gs.MayhemDiscards[card] == gs.Turn
}

// ClearMayhemDiscards drops all per-card discard turn records. Called in
// EndOfTurnCleanup so the "this turn" window closes — a card discarded on
// turn T cannot be mayhem-cast on turn T+1 even if it remains in the
// graveyard.
func ClearMayhemDiscards(gs *GameState) {
	if gs == nil {
		return
	}
	gs.MayhemDiscards = nil
}

// ---------------------------------------------------------------------------
// CastMayhem
// ---------------------------------------------------------------------------

// CastMayhem casts a card from `seatIdx`'s graveyard for its mayhem cost.
// CR §702.187a.
//
// Preconditions enforced here:
//   - card has the mayhem keyword
//   - card is in `seatIdx`'s graveyard
//   - card was discarded on the current turn (gs.MayhemDiscards[card] == gs.Turn)
//   - seat can afford `mayhemCost` mana (pass -1 to use the printed MayhemCost)
//
// On success the card is removed from the graveyard, mana is paid, and a
// StackItem is pushed with CostMeta["exile_on_resolve"]=true so the
// existing ResolveStackTop hook (stack.go) routes the card to exile after
// resolution per CR §702.187c. The seat-level flag
// "spell_mayhem_cast_this_turn:<seat>" is set for cards/triggers that key
// off "if you cast a card with mayhem this turn."
func CastMayhem(gs *GameState, seatIdx int, card *Card, mayhemCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasMayhem(card) {
		return nil, &CastError{Reason: "no_mayhem_keyword"}
	}
	if !MayhemEligible(gs, card) {
		return nil, &CastError{Reason: "not_discarded_this_turn"}
	}
	if mayhemCost < 0 {
		mayhemCost = MayhemCost(card)
	}
	if mayhemCost < 0 {
		return nil, &CastError{Reason: "invalid_mayhem_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the mayhem alternative
	// cost too (mayhem casts from the graveyard).
	mayhemCost = EffectiveAlternativeCost(gs, card, seatIdx, mayhemCost, CastContext{FromGraveyard: true})
	if seat.ManaPool < mayhemCost {
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
	if !removeFromZone(seat, card, ZoneGraveyard) {
		return nil, &CastError{Reason: "not_in_graveyard"}
	}
	seat.ManaPool -= mayhemCost
	SyncManaAfterSpend(seat)
	if mayhemCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: mayhemCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "mayhem_cast",
				"keyword": "mayhem",
				"rule":    "601.2f",
			},
		})
	}
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"exile_on_resolve":  true,
			"zone_cast_keyword": "mayhem",
			"mayhem":            true,
			"mayhem_cost":       mayhemCost,
		},
	}
	PushStackItem(gs, item)

	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_mayhem_cast_this_turn:"+itoa(seatIdx)] = 1

	// Consume the per-card eligibility — a card can only be mayhem-cast
	// once even if it returns to the graveyard later this turn (the new
	// graveyard arrival is not the same "discarded it this turn" event).
	delete(gs.MayhemDiscards, card)

	gs.LogEvent(Event{
		Kind:   "mayhem_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: mayhemCost,
		Details: map[string]interface{}{
			"rule": "702.187a",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellMayhemCastThisTurn returns true if any spell was cast for its
// mayhem cost by `seatIdx` during the current turn.
func SpellMayhemCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_mayhem_cast_this_turn:"+itoa(seatIdx)] > 0
}
