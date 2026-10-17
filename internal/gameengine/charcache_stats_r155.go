package gameengine

import "sync/atomic"

// r155 — charCache hit accounting.
//
// BaseCharacteristics is the steady-state allocation top (25.59% cum,
// 18.51% flat) and `c := &Characteristics{` inside it was 2.79GB of the
// r153 burst. That allocation is NOT removable: GetEffectiveCharacteristics
// stashes the struct in gs.charCache and returns the pointer, so it is
// retained by both the cache and the caller.
//
// Which makes the interesting question a different one. There IS a cache.
// If BaseCharacteristics still dominates, the cache must be missing most of
// the time — InvalidateCharacteristicsCache bumps a GLOBAL epoch and has
// 371 production call sites. But "must be" is a guess, and the charCache
// split is a correctness-critical round that would be designed on it.
//
// So measure it first, in production, before designing anything. The
// counters are plain uint64 on the GameState — a game is single-goroutine,
// so the hot path pays no atomic — and are published into process-wide
// atomics once per game, mirroring TriggerLimitSuppressions and
// RolloutClones.
var (
	charCacheHitsTotal   atomic.Uint64
	charCacheMissesTotal atomic.Uint64
	charCacheInvalTotal  atomic.Uint64
)

// CharCacheCounters returns the process-wide cumulative hit, miss and
// invalidation counts. Deltas are the per-window figures.
func CharCacheCounters() (hits, misses, invalidations uint64) {
	return charCacheHitsTotal.Load(), charCacheMissesTotal.Load(), charCacheInvalTotal.Load()
}

// PublishCharCacheStats folds this game's counters into the process-wide
// totals and resets them. Call once per completed game. Publishing rather
// than letting a reader reach into another worker's GameState is the r133
// rule: publish, don't inspect.
func (gs *GameState) PublishCharCacheStats() {
	if gs == nil {
		return
	}
	if gs.ccHits > 0 {
		charCacheHitsTotal.Add(gs.ccHits)
	}
	if gs.ccMisses > 0 {
		charCacheMissesTotal.Add(gs.ccMisses)
	}
	if gs.ccInval > 0 {
		charCacheInvalTotal.Add(gs.ccInval)
	}
	gs.ccHits, gs.ccMisses, gs.ccInval = 0, 0, 0
}

// ResetCharCacheCountersForTest zeroes the process-wide totals so a test
// can delta them without depending on whatever ran before it.
func ResetCharCacheCountersForTest() {
	charCacheHitsTotal.Store(0)
	charCacheMissesTotal.Store(0)
	charCacheInvalTotal.Store(0)
}
