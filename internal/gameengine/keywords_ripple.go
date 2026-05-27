package gameengine

// keywords_ripple.go — Ripple (CR §702.92, Coldsnap 2006) as a real
// cast-on-cast cascade-into-free mechanic.
//
// CR §702.92a: Ripple is a triggered ability. "Ripple N" means "When
//              you cast this spell, you may reveal the top N cards of
//              your library. You may cast any cards revealed this way
//              with the same name as this spell without paying their
//              mana costs. Then put the rest on the bottom of your
//              library in any order."
//
// Implementation: ApplyRipple is invoked when a spell with ripple is
// cast. It reveals the top N of the caster's library, casts each
// name-matching card for free (StackItem.CostMeta["ripple_cast"]=true),
// and puts the remaining cards on the bottom of the library in their
// revealed order (the printed text says "in any order"; we preserve the
// reveal order for determinism — that's a valid concrete choice within
// the "any order" allowance).
//
// Chain behavior: each ripple-cast card, if it itself has ripple, fires
// its own ripple trigger ("when you cast this spell"). The chain is
// modeled by ApplyRipple recursing after pushing each free cast. CR
// §603.3 stack ordering of nested triggers is not strictly emulated —
// this is the same pragmatic simplification cascade.go uses for nested
// cascades.

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// ---------------------------------------------------------------------------
// HasRipple / RippleN
// ---------------------------------------------------------------------------

// HasRipple returns true if the card has the ripple keyword in its AST.
func HasRipple(card *Card) bool {
	_, ok := RippleN(card)
	return ok
}

// RippleN returns the N (number of cards revealed) for the card's
// ripple keyword. Returns (N, true) if present, (0, false) otherwise.
// Accepts the arg as either float64 (JSON) or int.
func RippleN(card *Card) (int, bool) {
	if card == nil || card.AST == nil {
		return 0, false
	}
	for _, ab := range card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kw.Name), "ripple") {
			continue
		}
		n := 0
		if len(kw.Args) > 0 {
			switch v := kw.Args[0].(type) {
			case float64:
				n = int(v)
			case int:
				n = v
			}
		}
		return n, true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// Stack predicate
// ---------------------------------------------------------------------------

// IsRippleCast reports whether a StackItem was put on the stack by a
// ripple trigger (cast for free without paying mana cost).
func IsRippleCast(item *StackItem) bool {
	if item == nil || item.CostMeta == nil {
		return false
	}
	v, ok := item.CostMeta["ripple_cast"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// ---------------------------------------------------------------------------
// ApplyRipple
// ---------------------------------------------------------------------------

// ApplyRipple resolves the ripple trigger for a spell named
// `sourceCard.Name` cast by `casterSeat`. Reveals the top `n` cards of
// the caster's library, casts each name-matching card for free, then
// puts the remaining revealed cards on the bottom of the library in
// reveal order.
//
// Returns the number of free casts performed (including any chained
// from recursive ripples).
//
// Preconditions:
//   - gs and sourceCard non-nil
//   - casterSeat in range
//   - n >= 0 (n == 0 is a valid no-op)
//
// Side effects:
//   - "ripple_trigger" event logged when the trigger fires
//   - "ripple_reveal" event logged with the revealed-card names
//   - "ripple_hit" event logged per name match (before each free cast)
//   - each free cast pushes a StackItem with CostMeta:
//       ripple_cast       = true
//       ripple_source     = sourceCard (the *Card that triggered)
//       ripple_n          = n (the revealed count for that trigger)
//   - matched cards are removed from the library; non-matched cards are
//     appended to the library bottom in their reveal order
//   - the free-cast StackItem is resolved immediately to commit the
//     "cast and resolve" effect; if the cast card itself has ripple,
//     ApplyRipple is invoked recursively to model the chain
func ApplyRipple(gs *GameState, casterSeat int, sourceCard *Card, n int) int {
	if gs == nil || sourceCard == nil {
		return 0
	}
	if casterSeat < 0 || casterSeat >= len(gs.Seats) {
		return 0
	}
	if n <= 0 {
		return 0
	}
	seat := gs.Seats[casterSeat]
	if seat == nil {
		return 0
	}

	// Reveal: pop up to n cards from the top of the library into a
	// snapshot slice. We pop rather than peek because matched cards need
	// to leave the library to be cast, and the remaining ones go on the
	// bottom — neither stays at the top, so it's cleaner to remove all
	// at once and re-bottom the leftovers.
	limit := n
	if limit > len(seat.Library) {
		limit = len(seat.Library)
	}
	revealed := make([]*Card, limit)
	copy(revealed, seat.Library[:limit])
	seat.Library = seat.Library[limit:]

	gs.LogEvent(Event{
		Kind:   "ripple_trigger",
		Seat:   casterSeat,
		Source: sourceCard.DisplayName(),
		Amount: n,
		Details: map[string]interface{}{
			"rule": "702.92a",
		},
	})

	revealedNames := make([]string, 0, len(revealed))
	for _, c := range revealed {
		if c == nil {
			revealedNames = append(revealedNames, "")
			continue
		}
		revealedNames = append(revealedNames, c.DisplayName())
	}
	gs.LogEvent(Event{
		Kind:   "ripple_reveal",
		Seat:   casterSeat,
		Source: sourceCard.DisplayName(),
		Amount: len(revealed),
		Details: map[string]interface{}{
			"revealed":      revealedNames,
			"revealed_count": len(revealed),
		},
	})

	wantName := strings.ToLower(strings.TrimSpace(sourceCard.Name))
	matchedCount := 0

	// Track which revealed cards are matches in the reveal-order slice
	// so unmatched ones can be re-bottomed in that same order.
	matched := make([]bool, len(revealed))
	for i, c := range revealed {
		if c == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(c.Name)) != wantName {
			continue
		}
		matched[i] = true
	}

	// Cast each match for free, in reveal order.
	for i, c := range revealed {
		if !matched[i] || c == nil {
			continue
		}
		gs.LogEvent(Event{
			Kind:   "ripple_hit",
			Seat:   casterSeat,
			Source: c.DisplayName(),
			Details: map[string]interface{}{
				"trigger_source": sourceCard.DisplayName(),
				"reveal_index":   i,
				"rule":           "702.92a",
			},
		})
		castOneRippleFree(gs, casterSeat, sourceCard, c, n)
		matchedCount++

		// Chain: if the just-cast card itself has ripple, fire its own
		// trigger now ("When you cast this spell").
		if rippleN, has := RippleN(c); has && rippleN > 0 {
			matchedCount += ApplyRipple(gs, casterSeat, c, rippleN)
		}
	}

	// Re-bottom unmatched revealed cards in reveal order. Per the rules
	// the controller chooses an order; we pick the reveal order for
	// determinism (testable, matches the natural reading of "in any
	// order" — controller's choice is "the order I revealed them").
	for i, c := range revealed {
		if matched[i] || c == nil {
			continue
		}
		seat.Library = append(seat.Library, c)
	}

	return matchedCount
}

// castOneRippleFree pushes the ripple-free StackItem for `card` and
// resolves it immediately. The cast item carries CostMeta identifying
// it as a ripple cast and pointing back at the trigger source.
func castOneRippleFree(gs *GameState, casterSeat int, sourceCard, card *Card, n int) {
	if gs == nil || card == nil {
		return
	}
	eff := collectSpellEffect(card)
	item := &StackItem{
		Controller: casterSeat,
		Card:       card,
		CastZone:   ZoneLibrary,
		Effect:     eff,
		CostMeta: map[string]interface{}{
			"ripple_cast":   true,
			"ripple_source": sourceCard,
			"ripple_n":      n,
		},
	}
	// (cascade.go mangles the card's Name to suffix " (cascade)"; we
	// intentionally don't replicate that — mutating Card.Name corrupts
	// downstream name-match chains and any name-keyed analytics. The
	// CostMeta stamp above is the authoritative marker.)
	PushStackItem(gs, item)

	// CR §117.3 / §117.4 — opponents receive priority once a spell is
	// on the stack, BEFORE it resolves. PriorityRound polls APNAP so
	// counters and instant-speed responses can be cast against the
	// ripple-cast spell. The guarded ResolveStackTop below intentionally
	// skips resolution when the stack top changed (a counter is on top)
	// so the outer DrainStack handles the response.
	PriorityRound(gs)
	if len(gs.Stack) > 0 && gs.Stack[len(gs.Stack)-1] == item {
		ResolveStackTop(gs)
		StateBasedActions(gs)
	}
}
