package gameengine

// keywords_flashback.go — Flashback (CR §702.34) as a real cast-from-graveyard
// mechanic.
//
// CR §702.34a: Flashback is a static ability that functions while the card
//              with flashback is in a player's graveyard. "Flashback [cost]"
//              means "You may cast this card from your graveyard by paying
//              [cost] rather than paying its mana cost."
// CR §702.34b: Casting a spell using its flashback ability follows the rules
//              for paying alternative costs in §601.2b and §601.2f-h.
// CR §702.34c: If a spell with flashback would be put into a graveyard from
//              the stack, exile it instead.
//
// Implementation mirrors the warp pattern in keywords_batch6.go: thin
// helpers (HasFlashback, FlashbackCost), a cast entry point (CastFlashback)
// that removes the card from its owner's graveyard, pays the flashback
// cost, and pushes a StackItem flagged with CostMeta["exile_on_resolve"] =
// true. The exile-instead-of-graveyard hook in stack.go (ResolveStackTop,
// ShouldExileOnResolve branch) routes the card to exile on resolution.
//
// Snapcaster Mage-style "target instant or sorcery card in your graveyard
// gains flashback until end of turn" is implemented by
// GrantFlashbackUntilEOT, which registers a ZoneCastPermission via the
// existing ZoneCastGrants map with Duration="until_end_of_turn".

import (
	"github.com/hexdek/hexdek/internal/gameast"
	"github.com/hexdek/hexdek/internal/mana"
)

// ---------------------------------------------------------------------------
// HasFlashback / FlashbackCost
// ---------------------------------------------------------------------------

// HasFlashback returns true if the card has the flashback keyword in its
// AST.
func HasFlashback(card *Card) bool {
	return cardHasKeywordByName(card, "flashback")
}

// FlashbackCost returns the converted mana cost of the flashback keyword's
// alternative cost. Accepts the keyword arg as either a mana string
// ("{2}{R}{R}") or a plain numeric value. Returns 0 if the keyword is
// absent or the args are malformed; callers should treat 0 as "free" only
// when they have positively confirmed HasFlashback.
func FlashbackCost(card *Card) int {
	if card == nil || card.AST == nil {
		return 0
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !keywordNameEquals(kw, "flashback") {
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

// keywordNameEquals matches a keyword name case-insensitively without
// importing strings repeatedly at call sites.
func keywordNameEquals(kw *gameast.Keyword, want string) bool {
	if kw == nil {
		return false
	}
	return equalFoldTrimmed(kw.Name, want)
}

func equalFoldTrimmed(a, b string) bool {
	// Trim ASCII whitespace then case-fold; keyword names are ASCII.
	for len(a) > 0 && (a[0] == ' ' || a[0] == '\t') {
		a = a[1:]
	}
	for len(a) > 0 && (a[len(a)-1] == ' ' || a[len(a)-1] == '\t') {
		a = a[:len(a)-1]
	}
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// CastFlashback
// ---------------------------------------------------------------------------

// CastFlashback casts a card from `seatIdx`'s graveyard for its flashback
// cost. CR §702.34a.
//
// Preconditions enforced here:
//   - card has the flashback keyword OR a ZoneCastGrant with Keyword=="flashback"
//     is active for it (Snapcaster grant path)
//   - card is in `seatIdx`'s graveyard
//   - seat can afford `flashbackCost` mana (pass -1 to use the printed
//     FlashbackCost; for granted flashback pass the grant's ManaCost)
//
// On success the card is removed from the graveyard, mana is paid, and a
// StackItem is pushed with CostMeta["exile_on_resolve"]=true so the
// existing ResolveStackTop hook (stack.go) routes the card to exile after
// resolution per CR §702.34c. The seat-level flag
// "spell_flashbacked_this_turn:<seat>" is set for cards that key off
// "if a card with flashback was cast this turn" (Past in Flames-adjacent
// triggers).
//
// Returns (CostPaymentResult, error). The result is intentionally minimal
// — callers that need the full cast pipeline (cast triggers, storm,
// priority round) should call CastFromZone with NewFlashbackPermission
// instead. This entry point mirrors CastWarp: pay + push, leaving stack
// resolution to the caller.
func CastFlashback(gs *GameState, seatIdx int, card *Card, flashbackCost int) (*CostPaymentResult, error) {
	if gs == nil {
		return nil, &CastError{Reason: "nil game"}
	}
	if seatIdx < 0 || seatIdx >= len(gs.Seats) {
		return nil, &CastError{Reason: "invalid seat"}
	}
	if card == nil {
		return nil, &CastError{Reason: "nil card"}
	}
	// Permission check: either the card has the printed keyword, OR a
	// per-card flashback grant has been registered (Snapcaster path),
	// OR a graveyard-wide flashback grant applies (Iroh / Lier path).
	hasIntrinsic := HasFlashback(card)
	grant := GetZoneCastGrant(gs, card)
	hasGrant := grant != nil && grant.Keyword == "flashback" && grant.Zone == ZoneGraveyard
	graveCost, hasGraveGrant := EffectiveFlashbackCostFromGraveyardGrants(gs, seatIdx, card)
	if !hasIntrinsic && !hasGrant && !hasGraveGrant {
		return nil, &CastError{Reason: "no_flashback_permission"}
	}
	// Resolve cost: caller may pass -1 to mean "use the printed flashback
	// cost (or the grant's cost)". Per-card grant beats intrinsic; the
	// graveyard-wide grant is consulted only when neither of the prior
	// sources supply a cost (matches the "lowest castable cost wins"
	// expectation for stacked sources).
	if flashbackCost < 0 {
		switch {
		case hasGrant && grant.ManaCost >= 0:
			flashbackCost = grant.ManaCost
		case hasIntrinsic:
			flashbackCost = FlashbackCost(card)
		case hasGraveGrant:
			flashbackCost = graveCost
		}
	}
	if flashbackCost < 0 {
		return nil, &CastError{Reason: "invalid_flashback_cost"}
	}
	seat := gs.Seats[seatIdx]
	if seat == nil {
		return nil, &CastError{Reason: "nil seat"}
	}
	// Grant controller restriction (Snapcaster targets a single player's
	// graveyard; the grant's RequireController must match).
	if hasGrant && grant.RequireController >= 0 && grant.RequireController != seatIdx {
		return nil, &CastError{Reason: "wrong_controller_for_grant"}
	}
	if seat.ManaPool < flashbackCost {
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
	// Pay flashback cost.
	seat.ManaPool -= flashbackCost
	SyncManaAfterSpend(seat)
	if flashbackCost > 0 {
		gs.LogEvent(Event{
			Kind:   "pay_mana",
			Seat:   seatIdx,
			Amount: flashbackCost,
			Source: card.DisplayName(),
			Details: map[string]interface{}{
				"reason":  "flashback_cast",
				"keyword": "flashback",
				"rule":    "601.2f",
			},
		})
	}
	// Push onto the stack flagged for resolve-time exile.
	item := &StackItem{
		Card:       card,
		Controller: seatIdx,
		CastZone:   ZoneGraveyard,
		Effect:     collectSpellEffect(card),
		CostMeta: map[string]interface{}{
			"exile_on_resolve":  true,
			"zone_cast_keyword": "flashback",
			"flashback":         true,
			"flashback_cost":    flashbackCost,
		},
	}
	PushStackItem(gs, item)

	// Mark the seat as having flashbacked a spell this turn — used by
	// cards/triggers that key off "if you cast a card with flashback this
	// turn." Cleared in cleanup alongside other "this turn" flags.
	if gs.Flags == nil {
		gs.Flags = map[string]int{}
	}
	gs.Flags["spell_flashbacked_this_turn:"+itoa(seatIdx)] = 1

	// One-shot grant: if cast via Snapcaster-style grant, remove it now —
	// the card is leaving the graveyard, the grant has been consumed.
	if hasGrant {
		RemoveZoneCastGrant(gs, card)
	}

	gs.LogEvent(Event{
		Kind:   "flashback_cast",
		Seat:   seatIdx,
		Source: card.DisplayName(),
		Amount: flashbackCost,
		Details: map[string]interface{}{
			"rule": "702.34a",
		},
	})

	return &CostPaymentResult{}, nil
}

// SpellFlashbackedThisTurn returns true if any spell was cast for its
// flashback cost by `seatIdx` during the current turn.
func SpellFlashbackedThisTurn(gs *GameState, seatIdx int) bool {
	if gs == nil || gs.Flags == nil {
		return false
	}
	return gs.Flags["spell_flashbacked_this_turn:"+itoa(seatIdx)] > 0
}

// ---------------------------------------------------------------------------
// GrantFlashbackUntilEOT — Snapcaster Mage
// ---------------------------------------------------------------------------

// GrantFlashbackUntilEOT grants flashback to a target instant or sorcery
// card in `targetSeat`'s graveyard until end of turn. The flashback cost
// equals the card's mana cost (CR §702.34a — Snapcaster Mage's reminder
// text: "the flashback cost is equal to its mana cost").
//
// Implementation: registers a ZoneCastPermission keyed on the card pointer
// in gs.ZoneCastGrants with Duration="until_end_of_turn". The existing
// ExpireZoneCastGrants cleanup (phases.go EndOfTurnCleanup) removes it at
// turn end.
//
// `sourceName` should be the granting card's name (e.g. "Snapcaster Mage")
// for log/observability purposes.
func GrantFlashbackUntilEOT(gs *GameState, card *Card, targetSeat int, sourceName string) {
	grantFlashbackUntilEOTInternal(gs, card, targetSeat, sourceName, -1)
}

// GrantFlashbackUntilEOTWithCost is the same single-target flashback grant
// as GrantFlashbackUntilEOT, but with an explicit flashback cost override.
// Used by cards whose grant specifies a non-mana-cost flashback cost — e.g.
// Archmage's Newt (flashback {0} when saddled), The Fugitive Doctor
// (flashback {2}{R}{G} after sacrificing a Clue). Pass cost as the total
// mana value (the engine's ManaCost is a single int). Negative cost is
// treated as "use the card's printed mana cost", matching the default
// GrantFlashbackUntilEOT behavior.
func GrantFlashbackUntilEOTWithCost(gs *GameState, card *Card, targetSeat int, sourceName string, cost int) {
	grantFlashbackUntilEOTInternal(gs, card, targetSeat, sourceName, cost)
}

func grantFlashbackUntilEOTInternal(gs *GameState, card *Card, targetSeat int, sourceName string, overrideCost int) {
	if gs == nil || card == nil {
		return
	}
	if targetSeat < 0 || targetSeat >= len(gs.Seats) {
		return
	}
	// Single-target flashback grants only target instants/sorceries.
	if !cardHasType(card, "instant") && !cardHasType(card, "sorcery") {
		gs.LogEvent(Event{
			Kind:   "flashback_grant_rejected",
			Seat:   targetSeat,
			Source: sourceName,
			Details: map[string]interface{}{
				"card":   card.DisplayName(),
				"reason": "not_instant_or_sorcery",
				"rule":   "702.34a",
			},
		})
		return
	}
	cost := overrideCost
	if cost < 0 {
		cost = manaCostOf(card)
	}
	perm := &ZoneCastPermission{
		Zone:              ZoneGraveyard,
		Keyword:           "flashback",
		ManaCost:          cost,
		ExileOnResolve:    true,
		RequireController: targetSeat,
		SourceName:        sourceName,
		Duration:          "until_end_of_turn",
		GrantTurn:         gs.Turn,
	}
	RegisterZoneCastGrant(gs, card, perm)
}
