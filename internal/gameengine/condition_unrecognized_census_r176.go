package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// r176 — make evalCondition's fail-closed promise readable.
//
// evalCondition's tail says, in a comment, that an unrecognized
// condition kind is logged "so the unhandled set is a measurable
// burn-down list instead of invisible wrongness". It is not. The log
// goes through gs.LogEvent into gs.EventLog, a per-game in-memory
// slice; the grinder runs EventLogNone and throws it away. `grep -c
// condition_unrecognized server.log` over the entire production log
// returns 0. Nobody has ever been able to read the list.
//
// This is the same defect r142 found for the trigger limiter and r160
// found for the nine loop guards: a counter that exists only in an
// event log the box that matters does not retain. The fix is the same
// shape, deliberately — a process-global atomic census, published in
// the grinder's periodic line next to guards= and guardtop=.
//
// A corpus scan put 197 genuine misses across 434 conditional-cast
// cards, dominated by "if" (107 cards). That is a CORPUS count. How
// often each kind is actually reached in play is the number that
// decides what to implement first, and it is exactly what this
// measures.
//
// CARDINALITY IS BOUNDED, for the same reason the loop-guard labels
// are: c.Kind comes from the parser, and nothing structurally stops a
// future grammar from emitting something unbounded. Past
// condUnkMax distinct kinds, further new kinds collapse into one
// overflow bucket. "~other" appearing at all is itself a finding.
const (
	condUnkMax      = 64
	condUnkOverflow = "~other"
)

var (
	condUnkCounts sync.Map     // kind -> *atomic.Uint64
	condUnkCard   atomic.Int64 // distinct kinds seen
)

// bumpConditionUnrecognized records one evaluation that fell through
// every case in evalCondition.
//
// This sits on the resolve path, which runs constantly, so the steady
// state is one sync.Map load and one atomic add — no allocation, no
// lock, no string building. It is also only reached on the MISS path:
// a recognized kind returns from its case and never gets here.
func bumpConditionUnrecognized(kind string) {
	if kind == "" {
		kind = "(empty)"
	}
	if v, ok := condUnkCounts.Load(kind); ok {
		v.(*atomic.Uint64).Add(1)
		return
	}
	// First sighting. Enforce the bound before creating an entry. The
	// check is racy under concurrency and that is fine: it is a safety
	// ceiling, not a quota, and overshooting by the number of live
	// grinder workers cannot matter.
	if condUnkCard.Load() >= condUnkMax {
		v, _ := condUnkCounts.LoadOrStore(condUnkOverflow, new(atomic.Uint64))
		v.(*atomic.Uint64).Add(1)
		return
	}
	v, loaded := condUnkCounts.LoadOrStore(kind, new(atomic.Uint64))
	if !loaded {
		condUnkCard.Add(1)
	}
	v.(*atomic.Uint64).Add(1)
}

// ConditionUnrecognizedCounts snapshots every condition kind that has
// failed closed at least once.
func ConditionUnrecognizedCounts() map[string]uint64 {
	out := map[string]uint64{}
	condUnkCounts.Range(func(k, v any) bool {
		if n := v.(*atomic.Uint64).Load(); n > 0 {
			out[k.(string)] = n
		}
		return true
	})
	return out
}

// ConditionUnrecognizedTop renders the n busiest unrecognized kinds as
// a log-line-friendly "kind:count" list, heaviest first with ties
// broken by name so the output is stable across runs. It returns ""
// when nothing has ever failed closed, so the caller omits the field
// entirely and the normal grinder line is unchanged.
func ConditionUnrecognizedTop(n int) string {
	counts := ConditionUnrecognizedCounts()
	if len(counts) == 0 || n <= 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > n {
		keys = keys[:n]
	}
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(counts[k], 10))
	}
	return b.String()
}

// ResetConditionUnrecognizedForTest clears the census and the
// cardinality bookkeeping so a test can exercise the overflow bound
// more than once in one process.
func ResetConditionUnrecognizedForTest() {
	condUnkCounts.Range(func(k, v any) bool {
		condUnkCounts.Delete(k)
		return true
	})
	condUnkCard.Store(0)
}
