package gameengine

import (
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// r182 — what does percard_dispatch_depth_cap actually COST a game?
//
// THE QUESTION. fireTrigger refuses to dispatch once trigger nesting
// passes 8: it logs the guard and returns WITHOUT running any handler.
// Each of those is a triggered ability that should have been put on
// the stack and was not — a rules hole, not a slowdown. r160 made the
// firings countable (process-wide), r162 attributed them to an event,
// r163 published the depth survival curve. None of those can tell
// "genuine runaway loop, correctly contained" from "ordinary deep
// cascade, silently truncated", because all three are WINDOW
// aggregates: 1,857 firings could be 1,857 games losing one dispatch
// each, or one game losing 1,857.
//
// THE DISCRIMINATOR IS PER-GAME SHAPE. A finite cascade's swallowed
// dispatches are bounded by the board: a handful per game, spread
// across many games, on many different events. A genuine mutual loop
// — the live-pool deck pairing Bloodthirsty Conqueror ("whenever an
// opponent loses life, you gain that much life") with Marauding
// Blight-Priest ("whenever you gain life, each opponent loses 1
// life") — concentrates: ONE game loses thousands, on the same two
// events over and over. That difference is visible in a per-game
// distribution and invisible in a window total.
//
// WHAT CANNOT BE MEASURED, AND WHY IT IS NOT HERE. "How deep WOULD the
// cascade have gone" is unobtainable without changing behaviour.
// Depth only climbs because a dispatch runs and nests; the cap returns
// before dispatching, so depth at the cap site is always exactly 9 and
// no handler exists to carry it to 10. Letting the counter "keep
// climbing" past 8 is not a counter change — it requires running the
// handler the cap exists to skip. r163's header says the same thing
// ("THAT MEASUREMENT CANNOT EXIST") and backs it with a probe: a
// deliberately infinite recursive handler fired the guard 4 times and
// recorded max depth 9. The nearest behaviour-neutral analogue is
// cascadeMax below — how many swallows one top-level cascade racks up
// before unwinding — which measures how hard the cascade keeps
// pushing against the cap line within a single logical chain. A loop
// re-enters the frontier without bound inside one cascade; a wide
// finite cascade is bounded by board size.
//
// COST. fireTrigger is the hottest path in the engine. Everything
// added to the UNCAPPED path is one int compare against a local the
// caller already loaded, plus (only at depth 1) a call that stores a
// zero into a field. Everything else runs only on the capped path,
// which is rare by construction. No allocation, no map growth per
// dispatch, no formatting.
//
// AGGREGATION reuses what exists rather than inventing a channel:
// process-wide atomics read by renderGrinderLine, exactly r160/r163's
// shape, and the per-game first-event attribution rides on r162's
// already-bounded (guard, label) table under its own pseudo-guard
// name, so the cardinality ceiling and the "~other" overflow bucket
// come for free.
//
// Clone note: CloneForRollout builds from an explicit field list, so a
// rollout clone starts at zero and is counted as its own game. Same
// trade r163 took; the grinder reports clones=0.

// DepthCapFirstEventGuard is the pseudo-guard name under which the
// r162 label table holds PER-GAME first-swallowed-event attribution.
// It is deliberately distinct from the real guard name
// "percard_dispatch_depth_cap", whose labels are per-DISPATCH: a
// single looping game can contribute thousands there and exactly one
// here.
const DepthCapFirstEventGuard = "percard_depth_cap_firstevent"

var (
	// depthCapGames counts games that swallowed at least one dispatch.
	depthCapGames atomic.Uint64
	// depthCapMixedGames counts games that swallowed two or more
	// DISTINCT events — a wide cascade rather than a tight loop.
	depthCapMixedGames atomic.Uint64
	// depthCapMaxPerGame is the largest per-game swallow total seen.
	depthCapMaxPerGame atomic.Uint64
	// depthCapCascadeMax is the largest number of swallows inside a
	// single top-level cascade seen in any game.
	depthCapCascadeMax atomic.Uint64
	// depthCapBuckets is a power-of-two survival curve over per-game
	// swallow totals: bucket i counts games that reached at least
	// 1<<i swallows. Read as "games losing >= N dispatches".
	depthCapBuckets [depthCapBucketCount]atomic.Uint64
)

// depthCapBucketCount bounds the curve at 2^31 swallows in one game,
// far past anything a real game can produce; the top bucket is a
// saturating catch-all rather than an out-of-range panic.
const depthCapBucketCount = 32

// NoteCascadeRoot marks the start of a new TOP-LEVEL dispatch cascade
// (nesting depth 1). It only resets the per-cascade swallow counter;
// it records nothing and cannot alter dispatch.
func (gs *GameState) NoteCascadeRoot() {
	if gs == nil {
		return
	}
	gs.depthCapCascade = 0
}

// NoteDepthCapSwallow records one dispatch refused by the per-card
// trigger depth cap. It is called from fireTrigger immediately before
// the early return that already exists, and does nothing but count:
// no handler lookup, no dispatch, no state the engine reads back.
func (gs *GameState) NoteDepthCapSwallow(event string) {
	if gs == nil {
		return
	}
	gs.depthCapSwallowed++
	n := gs.depthCapSwallowed

	if n == 1 {
		depthCapGames.Add(1)
		gs.depthCapFirstEvent = event
		// Per-GAME attribution via r162's bounded label table.
		bumpLoopGuardLabel(DepthCapFirstEventGuard, event)
	} else if !gs.depthCapMixed && event != gs.depthCapFirstEvent {
		gs.depthCapMixed = true
		depthCapMixedGames.Add(1)
	}

	// Survival curve: bump exactly once per power-of-two crossing, so
	// a game of n swallows does ~log2(n) atomic adds in total.
	if n&(n-1) == 0 {
		i := 0
		for v := n; v > 1; v >>= 1 {
			i++
		}
		if i >= depthCapBucketCount {
			i = depthCapBucketCount - 1
		}
		depthCapBuckets[i].Add(1)
	}

	bumpAtomicMax(&depthCapMaxPerGame, uint64(n))

	gs.depthCapCascade++
	bumpAtomicMax(&depthCapCascadeMax, uint64(gs.depthCapCascade))
}

// bumpAtomicMax raises dst to v when v is larger, and does nothing
// otherwise. The load-first guard means the common case is one atomic
// load; the CAS loop runs only while a new record is being set.
func bumpAtomicMax(dst *atomic.Uint64, v uint64) {
	for {
		cur := dst.Load()
		if v <= cur {
			return
		}
		if dst.CompareAndSwap(cur, v) {
			return
		}
	}
}

// DepthCapStats snapshots the per-game swallow accounting.
// buckets maps a threshold N (a power of two) to the number of games
// that swallowed at least N dispatches.
func DepthCapStats() (games, mixed, maxPerGame, cascadeMax uint64, buckets map[uint64]uint64) {
	buckets = map[uint64]uint64{}
	for i := 0; i < depthCapBucketCount; i++ {
		if n := depthCapBuckets[i].Load(); n > 0 {
			buckets[uint64(1)<<uint(i)] = n
		}
	}
	return depthCapGames.Load(), depthCapMixedGames.Load(),
		depthCapMaxPerGame.Load(), depthCapCascadeMax.Load(), buckets
}

// DepthCapSummary renders the accounting for the grinder line, or ""
// when the cap has never fired so the caller omits the field entirely.
//
// Shape: games:932,max:1842,cascademax:57,mixed:88,hist[1:932,2:700,4:300]
func DepthCapSummary() string {
	games, mixed, maxPerGame, cascadeMax, buckets := DepthCapStats()
	if games == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("games:")
	b.WriteString(strconv.FormatUint(games, 10))
	b.WriteString(",max:")
	b.WriteString(strconv.FormatUint(maxPerGame, 10))
	b.WriteString(",cascademax:")
	b.WriteString(strconv.FormatUint(cascadeMax, 10))
	b.WriteString(",mixed:")
	b.WriteString(strconv.FormatUint(mixed, 10))
	if len(buckets) > 0 {
		thresholds := make([]uint64, 0, len(buckets))
		for t := range buckets {
			thresholds = append(thresholds, t)
		}
		sort.Slice(thresholds, func(i, j int) bool { return thresholds[i] < thresholds[j] })
		b.WriteString(",hist[")
		for i, t := range thresholds {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(strconv.FormatUint(t, 10))
			b.WriteByte(':')
			b.WriteString(strconv.FormatUint(buckets[t], 10))
		}
		b.WriteByte(']')
	}
	return b.String()
}

// ResetDepthCapForTest clears the process-wide accounting.
func ResetDepthCapForTest() {
	depthCapGames.Store(0)
	depthCapMixedGames.Store(0)
	depthCapMaxPerGame.Store(0)
	depthCapCascadeMax.Store(0)
	for i := range depthCapBuckets {
		depthCapBuckets[i].Store(0)
	}
}

// DepthCapSwallowedCount is this ONE game's swallow total — the same
// counter the process-wide accounting above aggregates, read back
// per-game so a caller that knows which decks and which seed produced
// this GameState can attribute the swallows to them.
//
// WHY THIS EXISTS. Everything in this file above is a process-global
// aggregate. Production over 5,794 games said 138 of them (2.4%) hit
// the cap, between them swallowing 2,936 dispatches, worst game 334.
// It could not say WHICH 138. The grinder's "SLOW GAME" line already
// prints seed= and decks=; this accessor is the missing half of that
// join, and the reason it is a method on *GameState rather than
// another atomic is that the attribution has to be per-game by
// construction.
//
// COST. One field load behind a nil check. It is NOT on the fireTrigger
// path — it is read once per game, from a deferred log — so it adds
// nothing to dispatch. It is deliberately read-only: no reset, no
// side effect, nothing the engine can branch on.
func (gs *GameState) DepthCapSwallowedCount() int {
	if gs == nil {
		return 0
	}
	return gs.depthCapSwallowed
}
