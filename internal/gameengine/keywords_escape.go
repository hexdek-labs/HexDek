package gameengine

// keywords_escape.go — Escape (CR §702.148, Theros Beyond Death 2020).
//
// CR §702.148a: Escape is a static ability that functions while the
//               card with escape is in a player's graveyard.
//               "Escape — [cost], Exile N other cards from your
//               graveyard" means "You may cast this card from your
//               graveyard by paying [cost] and exiling N other cards
//               from your graveyard rather than paying its mana cost."
// CR §702.148b: A spell cast for its escape cost is exiled when it
//               leaves the stack rather than going to its owner's
//               graveyard.
//
// Engine model
// ------------
// CastWithEscape mirrors CastFlashback / CastWarp / CastOmen but
// composes two payments instead of one: the alt mana cost AND an
// exile-tax of N caller-chosen graveyard cards. The caller passes
// the specific exile targets so player choice is preserved (vs. the
// generic AdditionalCost auto-pick path used by global escape grants
// such as Underworld Breach via NewBreachEscapePermission).
//
// Wiring:
//   - HasEscape / EscapeCost / EscapeExileCount are thin keyword
//     readers.
//   - CastWithEscape validates the spell is in the caster's
//     graveyard, validates each exile target is in the same
//     graveyard AND is not the spell itself (§702.148a "other"),
//     pays the mana cost, exiles the chosen fodder, and pushes a
//     StackItem flagged with CostMeta{"escape_cast": true,
//     "escape_exile_count": N, "exile_on_resolve": true,
//     "zone_cast_keyword": "escape"} so the existing
//     ShouldExileOnResolve branch in stack.go routes the spell to
//     exile on resolution per §702.148b.
//   - A ZoneCastPermission with Zone=graveyard, Keyword="escape",
//     ManaCost=escapeCost, ExileOnResolve=true is registered on the
//     card pointer at cast time for any AI/Hat or replay layer that
//     scans grants. The grant is removed when the cast pipeline
//     consumes it (single-use).

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasEscape / EscapeCost / EscapeExileCount
// ---------------------------------------------------------------------------

// HasEscape reports whether the card has the escape keyword in its
// AST.
func HasEscape(card *Card) bool {
	return cardHasKeywordByName(card, "escape")
}

// EscapeCost returns the mana cost of the escape keyword (the
// "[cost]" portion). Accepts the first keyword arg as either a mana
// string ("{2}{B}{B}") or a plain numeric value. Returns 0 if the
// keyword is absent or the args are malformed.
//
// Some printed escape arg layouts pack the mana cost and the exile
// count in different positions; this reader probes Args[0] for the
// mana cost and EscapeExileCount probes for an int anywhere in Args.
// In practice the parser emits the mana string at Args[0] and the
// exile count at Args[1].
func EscapeCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "escape") {
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

// EscapeExileCount returns the N in "Exile N other cards from your
// graveyard." Probes the keyword args for the first integer-typed
// value (Args[1] in the canonical parser shape; Args[0] would be the
// mana string). Returns 0 when escape is absent or no integer arg
// is present.
func EscapeExileCount(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "escape") {
			continue
		}
		for _, arg := range kw.Args {
			switch v := arg.(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
		}
		// The parser drops the "exile N other cards" tail into the raw text
		// (real escape cards — Uro, Kroxa — carry ONLY the mana cost in Args),
		// so escape's defining additional cost read as 0 and NO cards were
		// exiled. Recover the count from the raw text. Anchor on the word
		// "exile" so a generic mana pip in the cost (e.g. {2}{R}) isn't
		// mistaken for the exile count.
		if kw.Raw != "" {
			low := strings.ToLower(kw.Raw)
			if idx := strings.Index(low, "exile"); idx >= 0 {
				if n, ok := firstNumber(low[idx+len("exile"):]); ok {
					return n
				}
			}
		}
		return 0
	}
	return 0
}

// ---------------------------------------------------------------------------
// CastWithEscape
// ---------------------------------------------------------------------------

// CastWithEscape casts a card from `seatIdx`'s graveyard for its
// escape cost: pays `escapeManaCost` mana AND exiles `exileTargets`
// (which must all be in seat's graveyard and must NOT include the
// spell being cast — §702.148a "other"). CR §702.148.
//
// Preconditions enforced here:
//   - card has the escape keyword
//   - card is in seat's graveyard
//   - exileTargets are all in seat's graveyard
//   - exileTargets does not contain `card` (§702.148a "other")
//   - exileTargets are unique (no double-counting one card)
//   - len(exileTargets) >= EscapeExileCount(card) — the spec wording
//     is "exile N cards"; the caller may pay strictly the required
//     amount or more, but never less. CastWithEscape enforces a
//     strict equality on the required count to keep semantics
//     unambiguous (the caller bills exactly the required exile-tax
//     it asked for via escapeExileCount, which defaults to
//     EscapeExileCount(card) when -1 is passed).
//   - seat can afford `escapeManaCost`. Pass -1 to use the printed
//     EscapeCost.
//
// On success: spell removed from graveyard, mana paid, each exile
// target moved graveyard→exile, StackItem pushed with full CostMeta
// stamps (escape_cast, escape_exile_count, exile_on_resolve,
// zone_cast_keyword), per-turn "spell_escaped_this_turn:<seat>" flag
// set, and a ZoneCastPermission registered on the card pointer for
// observability.
//
// Returns *CostPaymentResult on success with ExiledCards populated.
func CastWithEscape(
	gs *GameState,
	seatIdx int,
	card *Card,
	escapeManaCost int,
	exileTargets []*Card,
) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	if !HasEscape(card) {
		return nil, &CastError{Reason: "no_escape_keyword"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
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
	if escapeManaCost < 0 {
		escapeManaCost = EscapeCost(card)
	}
	if escapeManaCost < 0 {
		return nil, &CastError{Reason: "invalid_escape_mana_cost"}
	}
	requiredExile := EscapeExileCount(card)
	if requiredExile < 0 {
		return nil, &CastError{Reason: "invalid_escape_exile_count"}
	}
	if len(exileTargets) < requiredExile {
		return nil, &CastError{Reason: "insufficient_exile_targets"}
	}
	// Validate the spell is in seat's graveyard.
	if !cardInZone(seat, card, ZoneGraveyard) {
		return nil, &CastError{Reason: "not_in_graveyard"}
	}
	// Validate exile targets: each must be in graveyard, none may be
	// `card`, and the list must be unique (no double-counting). We
	// scan in O(N) with a small seen-set since exile counts are
	// typically <= 5 for printed cards.
	seen := map[*Card]bool{}
	for _, target := range exileTargets {
		if target == nil {
			return nil, &CastError{Reason: "nil_exile_target"}
		}
		if target == card {
			return nil, &CastError{Reason: "exile_target_is_spell"}
		}
		if seen[target] {
			return nil, &CastError{Reason: "duplicate_exile_target"}
		}
		seen[target] = true
		if !cardInZone(seat, target, ZoneGraveyard) {
			return nil, &CastError{Reason: "exile_target_not_in_graveyard"}
		}
	}
	if seat.ManaPool < escapeManaCost {
		return nil, &CastError{Reason: "insufficient_mana"}
	}

	// All preconditions satisfied — perform the cost payments.

	// Remove the spell from graveyard.
	if !removeFromZone(seat, card, ZoneGraveyard) {
		// Should be unreachable after cardInZone passed, but defend.
		return nil, &CastError{Reason: "not_in_graveyard"}
	}
	// Pay mana.
	seat.ManaPool -= escapeManaCost
	SyncManaAfterSpend(seat)
	if escapeManaCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: escapeManaCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "escape_cast",
				"keyword": "escape",
				"rule":    "601.2f",
			},
		})
	}
	// Exile the fodder. We take the strict-count interpretation —
	// exactly `requiredExile` cards are removed; any extras passed by
	// the caller are ignored (the caller can pre-trim to bill exactly
	// what they want). This avoids accidentally consuming more
	// graveyard cards than the rules text demands.
	exiled := make([]*Card, 0, requiredExile)
	for i := 0; i < requiredExile; i++ {
		target := exileTargets[i]
		if !removeFromZone(seat, target, ZoneGraveyard) {
			// Unreachable after validation, but if it happens we
			// log + skip rather than partial-rollback (the
			// pre-validation guard already caught this).
			continue
		}
		seat.Exile = append(seat.Exile, target)
		exiled = append(exiled, target)
		gs.LogEvent(Event{
			Kind:   "exile_for_cost",
			Seat:   seatIdx,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"target":  target.DisplayName(),
				"reason":  "escape_exile_tax",
				"keyword": "escape",
				"rule":    "702.148a",
			},
		})
	}

	// Push onto the stack with the full escape CostMeta stamps. The
	// exile_on_resolve flag hands off to stack.go's
	// ShouldExileOnResolve branch so the spell is routed to exile on
	// resolution per §702.148b.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"escape_cast":        true,
			"escape_exile_count": requiredExile,
			"exile_on_resolve":   true,
			"zone_cast_keyword":  "escape",
		},
	}
	PushStackItem(gs, item)

	// Mark the seat for "if you cast an escape spell this turn"
	// triggers (Kroxa-adjacent).
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_escaped_this_turn:"+itoa(seatIdx)] = 1

	// Register a ZoneCastPermission keyed on the card pointer so
	// observers / replay (Heimdall) see the grant even though the
	// cast bypassed the generic CastFromZone pipeline. The grant is
	// effectively single-use — Heimdall sees it appear and then the
	// card leaves graveyard, so it can be reaped opportunistically.
	RegisterZoneCastGrant(gs, card, NewEscapePermission(escapeManaCost, requiredExile))

	gs.LogEvent(Event{
		Kind:   "escape_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: escapeManaCost,
		Details: map[string]interface{}{
			"rule":         "702.148a",
			"exile_count":  requiredExile,
			"exiled_names": exileTargetNames(exiled),
		},
	})

	return &CostPaymentResult{ExiledCards: exiled}, nil
}

// ---------------------------------------------------------------------------
// Stack / per-turn predicates
// ---------------------------------------------------------------------------

// IsEscapeCast reports whether a StackItem was cast via escape.
func IsEscapeCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["escape_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// EscapeExileCountOfItem returns the per-cast exile-tax recorded on
// a stack item (the same value stamped by CastWithEscape). Returns 0
// when the item wasn't an escape cast or no count was stamped.
func EscapeExileCountOfItem(item *StackItem) int {
	if item == nil || item.CostMeta == nil {
		return 0
	}
	v, ok := item.CostMeta["escape_exile_count"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

// SpellEscapedThisTurn returns true if any spell was cast via escape
// by `seatIdx` during the current turn.
func SpellEscapedThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_escaped_this_turn:"+itoa(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// cardInZone reports whether the given card pointer is present in
// seat's named zone (currently used for hand / graveyard / exile).
func cardInZone(seat *Seat, card *Card, zone string) bool {
	if seat == nil || card == nil {
		return false
	}
	var slice []*Card
	switch zone {
	case ZoneGraveyard:
		slice = seat.Graveyard
	case ZoneHand:
		slice = seat.Hand
	case ZoneExile:
		slice = seat.Exile
	default:
		return false
	}
	for _, c := range slice {
		if c == card {
			return true
		}
	}
	return false
}

// exileTargetNames returns a stable []string of display names for
// logging. Allocated lazily (only when the event is built) so the
// hot path doesn't pay for it when no observer reads it.
func exileTargetNames(cards []*Card) []string {
	if len(cards) == 0 {
		return nil
	}
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		if c == nil {
			continue
		}
		out = append(out, c.DisplayName())
	}
	return out
}
