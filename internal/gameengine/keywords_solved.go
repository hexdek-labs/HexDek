package gameengine

// keywords_solved.go — Solved (CR §702.169) as a real designation toggle.
//
// CR §702.169a: Solved is a designation that an enchantment with one or
//               more "To solve" abilities can have. A Case enchantment
//               (or any object with a "To solve" rider) becomes solved
//               when its solve condition is met; static abilities printed
//               on the card that begin "Solved —" (or that key off the
//               solved state) function only while the object is solved.
// CR §702.169b: Once solved, an object remains solved for as long as it
//               stays on the battlefield. Re-entering the battlefield
//               clears the designation (CR §400.7).
//
// The runtime representation is a single per-permanent flag —
// perm.Flags["solved"] = 1 — set by MarkSolved and cleared by ClearSolved
// (LTB cleanup). Per-card handlers for cases (Murders at Karlov Manor,
// MKM) call MarkSolved from inside the trigger that detects their
// "To solve" condition. A "becomes solved" trigger fires through
// FireCardTrigger("became_solved", ...) so reactive abilities can hook
// it.

// IsSolved reports whether the permanent has the solved designation.
// CR §702.169a.
func IsSolved(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["solved"] > 0
}

// HasSolveAbility returns true if the card carries the "solved" keyword
// marker — used by the parser to flag a Case/Solve enchantment whose
// static abilities gate on the solved designation. Backwards-compatible
// shim for the original keyword stub.
func HasSolveAbility(card *Card) bool {
	return cardHasKeywordByName(card, "solved")
}

// MarkSolved flips a permanent into the solved state and fires a
// "became_solved" trigger so reactive cards can observe it. Idempotent:
// calling MarkSolved on an already-solved permanent is a no-op and emits
// no second trigger.
//
// CR §702.169b — once solved, a permanent stays solved until it leaves
// the battlefield. Per-card LTB handlers do not need to clear the flag
// (the Permanent leaves with the rest of its state); ClearSolved exists
// only for callers that want to reset a permanent in place (replacement
// effects that conceptually re-enter the object without a real LTB).
func MarkSolved(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil {
		return
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	if perm.Flags["solved"] > 0 {
		return
	}
	perm.Flags["solved"] = 1

	name := ""
	if perm.Card != nil {
		name = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "became_solved",
		Seat:   perm.Controller,
		Source: name,
		Details: map[string]interface{}{
			"rule": "702.169a",
		},
	})

	FireCardTrigger(gs, "became_solved", map[string]interface{}{
		"perm":            perm,
		"card_name":       name,
		"controller_seat": perm.Controller,
	})
}

// ClearSolved removes the solved designation from a permanent. Engine
// code should rarely call this; it exists for symmetric handling of
// replacement effects and tests.
func ClearSolved(perm *Permanent) {
	if perm == nil || perm.Flags == nil {
		return
	}
	delete(perm.Flags, "solved")
}
