package gameengine

import (
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// etb_vanishing_fading.go — generic §614.1c ETB self-replacement for the
// Vanishing (CR §702.63) and Fading (CR §702.32) keywords:
//
//	Vanishing N — "This permanent enters with N time counters on it."
//	Fading N    — "This permanent enters with N fade counters on it."
//
// Thor parses these as bare Keyword abilities ("vanishing 3" / "fading 2");
// the "enters with N <kind> counters" reminder text is NOT reconstructed into
// the AST, and no engine path read the keyword — so all ~36 Vanishing/Fading
// permanents entered with ZERO counters. That is a §614.1c replacement gap:
// the counters are the whole point of the mechanic (fade/time-counter sinks
// like Rusting Golem's P/T, "remove a fade counter" activations, and the
// upkeep countdown all read the starting count).
//
// This places only the ENTERS-WITH counters (the §614.1c replacement half).
// The upkeep "remove a counter, sacrifice when the last is gone" countdown is
// a separate triggered-ability / SBA gap and is intentionally out of scope
// here (see the audit report) — but seeding the correct starting counters is
// a strict correctness improvement over entering with none, and is required
// before any countdown handler could work.
//
// Invoked at the ETB chokepoint alongside ApplyStaticETBCounters, BEFORE the
// ETB trigger fan-out so observers (counter doublers, "enters with a counter"
// watchers) see the final state.
//
// CONSERVATIVE GATES (defer rather than double-place):
//   - card already carries a structured `etb_with_counters` Modification
//     (Thor parsed the reminder text) -> ApplyStaticETBCounters owns it
//   - card has a per-card ETB hook (e.g. Chronozoa adds its own time
//     counters) -> the snowflake owns it
//   - face-down permanent (CR §708.4) -> no abilities
//   - keyword count missing / non-positive -> skip
func ApplyVanishingFadingETBCounters(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	if perm.Flags != nil && perm.Flags["face_down"] != 0 {
		return
	}
	// Structured etb_with_counters path owns it — don't double-place.
	if cardHasETBCounterStatic(perm.Card) {
		return
	}
	// Per-card snowflake owns this card's ETB counters.
	if HasETBHook != nil && HasETBHook(perm.Card.DisplayName()) {
		return
	}
	placed := false
	for _, ab := range perm.Card.AST.Abilities {
		kw, ok := ab.(*gameast.Keyword)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(kw.Name))
		var counterKind string
		switch name {
		case "vanishing":
			counterKind = "time"
		case "fading":
			counterKind = "fade"
		default:
			continue
		}
		n := vanishingFadingCount(kw)
		if n <= 0 {
			// Vanishing/Fading always carry a positive N; a missing count is a
			// parse artifact — skip rather than guess.
			continue
		}
		perm.AddCounter(counterKind, n)
		placed = true
		gs.LogEvent(Event{
			Kind:   "etb_with_counters",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: n,
			Details: map[string]interface{}{
				"counter_kind": counterKind,
				"keyword":      name,
				"path":         "vanishing_fading_keyword",
			},
		})
	}
	if placed {
		gs.InvalidateCharacteristicsCache()
	}
}

// vanishingFadingCount extracts N from a Vanishing/Fading keyword ability.
// Prefers a structured Args[0] integer; falls back to the trailing number in
// the raw ("vanishing 3" -> 3).
func vanishingFadingCount(kw *gameast.Keyword) int {
	if len(kw.Args) > 0 {
		if n, ok := asInt(kw.Args[0]); ok && n > 0 {
			return n
		}
	}
	fields := strings.Fields(strings.ToLower(kw.Raw))
	for i := len(fields) - 1; i >= 0; i-- {
		if n, err := strconv.Atoi(fields[i]); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

// cardHasETBCounterStatic reports whether the card already carries a
// structured `etb_with_counters` Modification on a Static ability — in which
// case ApplyStaticETBCounters owns its starting counters and the keyword
// fallback must stand down to avoid double-placement.
func cardHasETBCounterStatic(card *Card) bool {
	if card == nil || card.AST == nil {
		return false
	}
	for _, ab := range card.AST.Abilities {
		if st, ok := ab.(*gameast.Static); ok && st.Modification != nil &&
			st.Modification.ModKind == "etb_with_counters" {
			return true
		}
	}
	return false
}
