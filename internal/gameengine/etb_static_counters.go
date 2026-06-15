package gameengine

import (
	"strings"

	"github.com/hexdek/hexdek/internal/gameast"
)

// hasDerivedCountClause reports whether an etb_with_counters arg list carries a
// derived-count clause (where_x: / for_each: / equal_to:) — meaning the "x"
// count is computed from board state (Voracious Wurm, Prime Speaker Zegana,
// Bioessence Hydra), NOT the spell's cast X.
func hasDerivedCountClause(args []interface{}) bool {
	for _, a := range args {
		s, ok := a.(string)
		if !ok {
			continue
		}
		s = strings.ToLower(s)
		if strings.HasPrefix(s, "where_x:") || strings.HasPrefix(s, "for_each:") ||
			strings.HasPrefix(s, "equal_to:") {
			return true
		}
	}
	return false
}

// ApplyStaticETBCounters walks the entering permanent's AST for Static
// abilities whose Modification.ModKind == "etb_with_counters" and applies
// the counter delta. Covers the generic templated phrase
// "this creature enters with N <kind> counter(s) on it" (CR §122.1g —
// self-replacement effect that modifies the act of entering, per §614.1d).
//
// Without this, cards with `Static{Modification: etb_with_counters}` but
// no per-card OnETB handler entered the battlefield with no counters at
// all — fine for "enters with one ability counter" cards that are
// nonsense at 0/0, but catastrophic for 0/0 creatures like District
// Mascot whose printed P/T is literally 0/0 and rely on the ETB counter
// to be playable. They survived as 0/0 creatures, and the SBA 704.5f
// kill window depended on whether the next state-based pass swept
// before the loki invariant check observed them.
//
// The generic effect-side handler at `resolveModificationEffect` (case
// "etb_with_counters") already existed for cases where the modification
// shows up as a resolving effect — but Static abilities don't resolve
// through the stack, so that handler was unreachable for District
// Mascot's shape. This primitive wires the Static→counter path.
//
// Args[0] is the count (int, default 1); Args[1] is the counter kind
// (string, default "+1/+1"). Matches `resolveModificationEffect`'s
// `etb_with_counters` arg shape so the same AST node works either way.
func ApplyStaticETBCounters(gs *GameState, perm *Permanent) {
	if gs == nil || perm == nil || perm.Card == nil || perm.Card.AST == nil {
		return
	}
	// Face-down permanents have no abilities (CR §708.4).
	if perm.Flags != nil && perm.Flags["face_down"] != 0 {
		return
	}
	for _, ab := range perm.Card.AST.Abilities {
		st, ok := ab.(*gameast.Static)
		if !ok || st.Modification == nil {
			continue
		}
		if st.Modification.ModKind != "etb_with_counters" {
			continue
		}
		// CR §702.33 — "if ~ was kicked, it enters with N counters." The
		// parser stamps a Condition (e.g. {kicked}) on the Static; honor
		// it so Grunn enters with 0 counters unkicked and N when kicked.
		// Unconditional statics (Condition == nil) always apply.
		if st.Condition != nil && !evalCondition(gs, perm, st.Condition) {
			continue
		}
		count := resolveETBCounterCount(perm, st.Modification.Args)
		if count <= 0 {
			continue
		}
		counterKind := "+1/+1"
		if len(st.Modification.Args) > 1 {
			if k, ok := st.Modification.Args[1].(string); ok && k != "" {
				counterKind = k
			}
		}
		PutCountersTriggered(gs, perm, counterKind, count, perm)
		gs.LogEvent(Event{
			Kind:   "etb_with_counters",
			Seat:   perm.Controller,
			Source: perm.Card.DisplayName(),
			Amount: count,
			Details: map[string]interface{}{
				"counter_kind": counterKind,
				"path":         "static_self_replacement",
			},
		})
	}
	gs.InvalidateCharacteristicsCache()
}

// resolveETBCounterCount resolves the count argument of an
// `etb_with_counters` modification against the entering permanent.
//
// Args[0] is normally a literal int ("enters with 3 +1/+1 counters").
// Some cards encode a VARIABLE count where the AST stamps the string
// "var" (e.g. Everflowing Chalice / Astral Cornucopia:
// Args = ["var", "charge", "for_each:time it was kicked"]). For those
// the count comes from how many times the spell was kicked, mirrored
// onto perm.Flags["multikick_count"] at ETB (0 when unkicked → 0
// counters, which is rules-correct). Shared by both the Static-ability
// path (ApplyStaticETBCounters) and the resolving-effect path
// (resolveModificationEffect case "etb_with_counters").
func resolveETBCounterCount(perm *Permanent, args []interface{}) int {
	if len(args) == 0 {
		return 1
	}
	// Variable count.
	if s, ok := args[0].(string); ok {
		switch s {
		case "x", "X":
			// "enters with X <counter> counters" where X is the spell's CHOSEN
			// X ({X}/{X}{X} cost): Chalice of the Void (charge), Walking
			// Ballista / Hangarback / every X-Hydra (+1/+1), Astral Cornucopia
			// (charge). These have NO kicker, so the old "x"→multikick_count
			// read returned 0 — they all entered with ZERO counters (the
			// reported Chalice bug, 81 cards corpus-wide). Read the chosen X,
			// mirrored onto perm.Flags["chosen_x"] at ETB.
			//
			// EXCEPTION: a derived-X clause (where_x: / for_each: / equal_to:)
			// means "x" is computed from board state, not the cast X — leave
			// those on the legacy path (handled elsewhere / stubbed) so this
			// fix doesn't mis-read them.
			if hasDerivedCountClause(args) {
				if perm != nil && perm.Flags != nil {
					return perm.Flags["multikick_count"]
				}
				return 0
			}
			if perm != nil && perm.Flags != nil {
				return perm.Flags["chosen_x"]
			}
			return 0
		case "var", "variable":
			// "for each time it was kicked" (Everflowing Chalice, Skitter of
			// Lizards, …) → multikicker count.
			if perm != nil && perm.Flags != nil {
				return perm.Flags["multikick_count"]
			}
			return 0
		}
	}
	if n, ok := asInt(args[0]); ok && n > 0 {
		return n
	}
	return 1
}
