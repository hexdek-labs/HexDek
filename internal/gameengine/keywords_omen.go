package gameengine

// keywords_omen.go — Omen, CR §720 (Omen Cards).
//
// CORRECTED r99 (was CLAUDE.md open issue (11)). Everything above this
// line used to describe a different mechanic. The file header read:
//
//	"Omen (CR §702.176) as a real cast-from-hand alt-cost mechanic that
//	 exiles the spell on resolution and grants a one-time
//	 cast-from-exile permission for the 'omen face.'"
//
// and quoted reminder text of the form "pay [cost] and exile it with an
// omen counter ... cast it on a later turn from exile", which is the
// foretell / plot shape, not omen's. Three things were wrong:
//
//  1. §702.176 IS IMPENDING. Omen is not a keyword ability and has no
//     §702 section at all; it is a two-part card frame defined in §720,
//     and the glossary entry is "Omen Card -> see rule 720". The wrong
//     number was in the doc comment AND emitted as the omen_cast
//     event's "rule" field, i.e. as event DATA.
//  2. THE RESOLUTION ZONE WAS WRONG. §720.3d: "As an Omen spell
//     resolves, its controller shuffles it into its owner's library
//     instead of putting it into its owner's graveyard as it resolves."
//     The engine exiled it.
//  3. THE CAST-FROM-EXILE GRANT IS FABRICATED. Nothing in §720 lets a
//     player cast the omen face later from exile; the card is in the
//     library. That grant is no longer registered.
//
// What §720 actually says, in the parts that bear on this file:
//
//	720.2   the inset frame defines ALTERNATIVE CHARACTERISTICS the
//	        object may have while it is a spell
//	720.3   as a player casts an omen card, they choose whether to cast
//	        it normally or as an Omen
//	720.3b  while on the stack as an Omen, the spell has ONLY its
//	        alternative characteristics
//	720.3d  as an Omen spell resolves, its controller shuffles it into
//	        its owner's library instead of putting it into its owner's
//	        graveyard
//	720.4   in every zone except the stack, and on the stack not as an
//	        Omen, the card has only its normal characteristics
//
// So the mechanic is a cast-time MODE CHOICE with a resolution-zone
// replacement, not an alternative cost with a delayed re-cast.
//
// Wiring now:
//   - HasOmen / OmenCost stay as the thin keyword readers the parser
//     feeds; the engine models the Omen half's cost through them.
//   - CastOmen removes the card from hand, pays, and pushes a StackItem
//     with CostMeta["shuffle_into_library_on_resolve"]=true so
//     ResolveStackTop's §720.3d branch sends the card to its owner's
//     library and shuffles.
//   - §720.3d is a RESOLUTION replacement only. A COUNTERED or fizzled
//     Omen spell is not covered by it and goes to its owner's graveyard
//     under the ordinary rule — postStackZoneForOffboard deliberately
//     does not consult the new flag.
//
// STILL NOT MODELLED, filed: §720.2/§720.3b alternative characteristics.
// The engine casts the card itself rather than an object wearing the
// Omen half's name, type line and text, so "while on the stack as an
// Omen the spell has ONLY its alternative characteristics" is not
// represented. Our oracle endpoint returns only the primary face for
// these cards, so the Omen half's text may not reach the parser at all
// — that is the thing to check first.
//
import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasOmen / OmenCost
// ---------------------------------------------------------------------------

// HasOmen reports whether the card has the omen keyword.
func HasOmen(card *Card) bool {
	return cardHasKeywordByName(card, "omen")
}

// OmenCost returns the converted mana cost of the omen keyword's
// alternative cost. The keyword arg may be a mana string ("{1}{G}") or
// a plain numeric value. Returns 0 if the keyword is absent or its args
// are malformed; callers should treat 0 as "free" only when they have
// positively confirmed HasOmen.
func OmenCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "omen") {
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
// CastOmen — pay the Omen-half cost from hand; §720.3d routes the card
// to its owner's library on resolution.
// ---------------------------------------------------------------------------

// CastOmen casts a card from `seatIdx`'s hand as an Omen.
// CR §720.3 / §720.3d.
//
// Preconditions enforced here:
//   - card has the omen keyword
//   - card is in `seatIdx`'s hand
//   - seat can afford `omenCost` mana (pass -1 to use the printed OmenCost)
//
// On success the card is removed from the hand, mana is paid, and a
// StackItem is pushed with CostMeta:
//
//	{
//	  "omen":              true,
//	  "omen_cost":         N,
//	  "zone_cast_keyword": "omen",
//	  "exile_on_resolve":  true,
//	}
//
// The CostMeta["exile_on_resolve"] flag is the canonical hand-off to
// stack.go's ResolveStackTop branch (zone_cast.go ShouldExileOnResolve)
// which routes the card to exile after resolution. Simultaneously a
// ZoneCastPermission is registered on the card pointer so the omen face
// may be cast from exile later at the card's printed mana cost — see
// NewOmenCastFromExilePermission for the shape of that grant.
//
// The seat-level flag "spell_omen_cast_this_turn:<seat>" is set for any
// card that keys off "if you cast a spell for its omen cost this turn."
func CastOmen(gs *GameState, seatIdx int, card *Card, omenCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasOmen(card) {
		return nil, &CastError{Reason: "no_omen_keyword"}
	}
	if omenCost < 0 {
		omenCost = OmenCost(card)
	}
	if omenCost < 0 {
		return nil, &CastError{Reason: "invalid_omen_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// CR §118.9 / §601.2f — cost modifiers apply to the omen alternative cost
	// too (the omen is cast from hand). Zero-value context.
	omenCost = EffectiveAlternativeCost(gs, card, seatIdx, omenCost, CastContext{})
	if seat.ManaPool < omenCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}
	// Remove from hand.
	if !removeFromZone(seat, card, ZoneHand) {
		return nil, &CastError{Reason: "not_in_hand"}
	}
	// Pay the omen cost.
	seat.ManaPool -= omenCost
	SyncManaAfterSpend(seat)
	if omenCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: omenCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "omen_cast",
				"keyword": "omen",
				"rule":    "601.2f",
			},
		})
	}
	// Push onto the stack flagged for resolve-time exile.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneHand,
		Effect:     collectSpellEffect(card),
		// §720.3d — shuffled into the OWNER's library as it resolves.
		// Deliberately NOT "exile_on_resolve", which is what this was
		// until r99 and is the wrong zone.
		CostMeta: map[string]interface{}{
			"shuffle_into_library_on_resolve": true,
			"zone_cast_keyword":               "omen",
			"omen":                            true,
			"omen_cost":                       omenCost,
		},
	}
	PushStackItem(gs, item)

	// Mark the seat as having omen-cast a spell this turn — used by
	// cards/triggers that key off "if you cast a card for its omen cost
	// this turn." Cleared in cleanup with other "this turn" flags.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_omen_cast_this_turn:"+itoa(seatIdx)] = 1

	// NO cast-from-exile grant (r99). This used to register one, on the
	// theory that omen exiles the card for a later re-cast. §720 grants
	// nothing of the kind and the card is shuffled into the library, so
	// the grant described an ability that does not exist.
	// NewOmenCastFromExilePermission now has NO PRODUCTION CALLERS and
	// is on the dead-code ledger; it is left in place rather than
	// deleted here because delete-or-wire is its own change.

	gs.LogEvent(Event{
		Kind:   "omen_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: omenCost,
		Details: map[string]interface{}{
			// §720.3 is the cast-time choice to cast the card as an
			// Omen. The old value here was "702.176", which is
			// Impending — a wrong number emitted as event DATA.
			"rule": "720.3",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellOmenCastThisTurn returns true if any spell was cast for its
// omen cost by `seatIdx` during the current turn.
func SpellOmenCastThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_omen_cast_this_turn:"+itoa(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// NewOmenCastFromExilePermission — the post-resolve "cast from exile" grant.
// ---------------------------------------------------------------------------

// NewOmenCastFromExilePermission — DEAD AS OF r99, no production
// callers. On the dead-code ledger; left in place because delete-or-wire
// is its own change.
//
// It returns a ZoneCastPermission that would let the card's owner cast
// the omen face from exile at the card's normal mana cost. The rule it
// used to cite for that ("cast it on a later turn from exile") does not
// exist: that is foretell/plot-shaped reminder text, and the number
// beside it pointed at Impending. An Omen spell is shuffled into its
// owner's library as it resolves, so there is never a card in exile for
// this grant to apply to.
//
// RequireController is the owner so an opponent that exiled the card
// could not hijack the second cast. Duration is empty.
func NewOmenCastFromExilePermission(owner int) *ZoneCastPermission {
	return &ZoneCastPermission{
		Zone:              ZoneExile,
		Keyword:           "omen",
		ManaCost:          -1, // use card's printed mana cost
		RequireController: owner,
		SourceName:        "omen_exile",
		Duration:          "", // permanent until cast or otherwise removed
	}
}
