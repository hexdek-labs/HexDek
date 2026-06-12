package gameengine

import "github.com/hexdek/hexdek/internal/gameast"

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
		perm.AddCounter(counterKind, count)
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
	// Variable count ("for each time it was kicked", etc.).
	if s, ok := args[0].(string); ok {
		switch s {
		case "var", "variable", "x", "X":
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
