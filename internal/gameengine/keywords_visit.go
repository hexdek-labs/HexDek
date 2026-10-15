package gameengine

// keywords_visit.go — Visit (CR §702.177) as a state-flag + trigger
// scaffold.
//
// Visit is the set-specific room/vehicle "tap to enter" mechanic. The
// trigger fires when a creature taps "into" a Room or Vehicle permanent
// (the act of saddling/crewing/entering); the visited permanent records
// the event in perm.Flags["visited_this_turn"] so other abilities
// printed on the same card or on triggered companion cards can read it.
//
// This file scaffolds the runtime hooks. Per-card handlers call
// ApplyVisit at the point of tap-into-room-or-vehicle; the engine sets
// the state flag, fires a "visited" trigger so reactive cards can hook
// it, and emits a "visit" event for the log. The visited_this_turn flag
// is cleared in EndOfTurnCleanup alongside other "this turn" perm flags.

// HasVisit returns true if the card has the visit keyword in its AST.
func HasVisit(card *Card) bool {
	return cardHasKeywordByName(card, "visit")
}

// VisitedThisTurn reports whether `perm` had at least one visit recorded
// during the current turn. Cleared in EndOfTurnCleanup.
func VisitedThisTurn(perm *Permanent) bool {
	if perm == nil || perm.Flags == nil {
		return false
	}
	return perm.Flags["visited_this_turn"] > 0
}

// ApplyVisit records a visit on `perm` made by `visitorSeat`. The
// state-flag set is what most card text gates on ("if this was visited
// this turn"); the trigger fan-out lets reactive companion cards hook
// the event. CR §702.177.
//
// Returns the new visited count for `perm` this turn. Callers that need
// to know "this is the Nth visit" (e.g. cards that reward repeated
// visits) can use the return value rather than re-scanning the flag.
func ApplyVisit(gs *GameState, visitorSeat int, perm *Permanent) int {
	if gs == nil || perm == nil {
		return 0
	}
	if perm.Flags == nil {
		perm.Flags = map[string]int{}
	}
	perm.Flags["visited_this_turn"]++
	count := perm.Flags["visited_this_turn"]

	name := ""
	if perm.Card != nil {
		name = perm.Card.DisplayName()
	}
	gs.LogEvent(Event{
		Kind:   "visit",
		Seat:   visitorSeat,
		Source: name,
		Amount: count,
		Details: map[string]interface{}{
			"visitor_seat":    visitorSeat,
			"controller_seat": perm.Controller,
			"visit_count":     count,
			"rule":            "702.159",
		},
	})

	FireCardTrigger(gs, "visited", map[string]interface{}{
		"perm":            perm,
		"card_name":       name,
		"visitor_seat":    visitorSeat,
		"controller_seat": perm.Controller,
		"visit_count":     count,
	})

	return count
}

// ClearVisitFlags removes the per-turn visited counter from every
// permanent on every battlefield. Called from EndOfTurnCleanup so the
// "visited this turn" window closes correctly.
func ClearVisitFlags(gs *GameState) {
	if gs == nil {
		return
	}
	for _, seat := range gs.Seats {
		if seat == nil {
			continue
		}
		for _, p := range seat.Battlefield {
			if p == nil || p.Flags == nil {
				continue
			}
			delete(p.Flags, "visited_this_turn")
		}
	}
}
