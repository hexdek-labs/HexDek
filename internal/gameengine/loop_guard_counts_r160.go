package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// r160 — loop-guard firings are published as process-wide counters so
// the grinder can report them.
//
// WHY THIS EXISTS. LogLoopGuardFired writes ONE place: the event log.
// The grinder runs EventPolicy = EventLogNone, whose LogEvent case is
// literally "Retain nothing". So every loop guard that fires in
// production — the only place these guards matter — leaves no trace
// whatsoever. Nine guards (move_no_progress, trigger_loop_cap,
// trigger_drain_cap, sba_max_passes, drain_iteration_cap, three
// percard_dispatch caps, and game_deadline) are all invisible on the
// box.
//
// This is not hypothetical. `grep -c game_deadline server.log` over the
// entire production log returns 0, while that same log contains games
// of 3102.6s, 2148.3s, 1536.7s, 1256.7s and 1162.0s against an ARMED
// 10-minute wall-clock cap (perGameWallClock, showmatch.go). Either the
// cap is not firing or it is firing and saying nothing, and until now
// there was no way to tell those apart.
//
// It also means a claim recorded in game_deadline.go's own header —
// that during the 2026-10-08 sixteen-day stall "zero loop guards had
// ever fired on that box" — was never verifiable. A guard that fired
// there would have left exactly the same evidence as one that did not.
//
// internal/gameengine deliberately imports no "log": it is a library
// and no file in it prints. So this publishes counters and lets the
// grinder's window line print them, the same shape r155 used for the
// charCache hit rate. Guards are rare by construction, so a sync.Map
// of atomics costs nothing on any path that matters.

var loopGuardCounts sync.Map // guard name -> *atomic.Uint64

func bumpLoopGuard(guard string) {
	if v, ok := loopGuardCounts.Load(guard); ok {
		v.(*atomic.Uint64).Add(1)
		return
	}
	v, _ := loopGuardCounts.LoadOrStore(guard, new(atomic.Uint64))
	v.(*atomic.Uint64).Add(1)
}

// LoopGuardCounts returns a snapshot of every guard that has fired at
// least once in this process, by name. Guards that never fired are
// absent rather than zero, so a caller can print nothing in the normal
// case.
func LoopGuardCounts() map[string]uint64 {
	out := map[string]uint64{}
	loopGuardCounts.Range(func(k, v any) bool {
		if n := v.(*atomic.Uint64).Load(); n > 0 {
			out[k.(string)] = n
		}
		return true
	})
	return out
}

// LoopGuardSummary renders the counts as a stable, sorted,
// log-line-friendly string: "game_deadline:2,sba_max_passes:1". It
// returns "" when nothing has fired, so the caller can omit the field
// entirely rather than print a permanent "guards=" noise column.
func LoopGuardSummary() string {
	counts := LoopGuardCounts()
	if len(counts) == 0 {
		return ""
	}
	names := make([]string, 0, len(counts))
	for k := range counts {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(n)
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(counts[n], 10))
	}
	return b.String()
}

// ResetLoopGuardCountsForTest clears every counter.
func ResetLoopGuardCountsForTest() {
	loopGuardCounts.Range(func(k, v any) bool {
		v.(*atomic.Uint64).Store(0)
		return true
	})
}
