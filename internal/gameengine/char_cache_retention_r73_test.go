package gameengine

import (
	"math/rand"
	"testing"
)

// r73 — the characteristics cache retained every permanent it ever saw.
//
// InvalidateCharacteristicsCache bumped charCacheEpoch, which makes every
// entry unreadable, but left the entries in the map. The map is keyed by
// *Permanent, so each dead entry kept a Permanent — and its Card,
// Modifications and counters — reachable for the whole life of the
// GameState. A token that entered and died on turn 3 was still live at
// turn 60, which shows up as GC mark cost rather than as a leak in the
// heap-in-use figure.
//
// Found while investigating a grinder throughput collapse. Pre-existing,
// not introduced by the r70/r72 keyword migration, but the migration made
// the cache hotter and so made it matter more.

func r73Game(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(7)), nil)
}

func TestCharCacheR73_DoesNotRetainDeadPermanents(t *testing.T) {
	gs := r73Game(t)
	const n = 500
	for i := 0; i < n; i++ {
		p := addBattlefield(gs, 0, "Spirit Token", 1, 1, "creature")
		_ = GetEffectiveCharacteristics(gs, p) // caches an entry for p
		gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
		gs.InvalidateCharacteristicsCache()
	}
	if got := len(gs.charCache); got != 0 {
		t.Errorf("cache retained %d entries for permanents that left the "+
			"battlefield; want 0", got)
	}
}

// Guard against the obvious over-correction: clearing must not break the
// cache. A query after an invalidation must still memoize, and a repeat
// query under the same epoch must still be served from the cache.
func TestCharCacheR73_StillCachesWithinAnEpoch(t *testing.T) {
	gs := r73Game(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")

	gs.InvalidateCharacteristicsCache()
	first := GetEffectiveCharacteristics(gs, p)
	if len(gs.charCache) != 1 {
		t.Fatalf("query after invalidation did not populate the cache (len=%d)",
			len(gs.charCache))
	}
	second := GetEffectiveCharacteristics(gs, p)
	if first != second {
		t.Error("second query under the same epoch was not served from the cache")
	}
}

// And that invalidation still invalidates — the clear must not be mistaken
// for the epoch bump doing the work.
func TestCharCacheR73_InvalidationStillRecomputes(t *testing.T) {
	gs := r73Game(t)
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	if got := gs.PowerOf(p); got != 2 {
		t.Fatalf("precondition: power %d, want 2", got)
	}
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", HandlerID: "r73_anthem",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, q *Permanent) bool { return q == p },
		ApplyFn: func(_ *GameState, _ *Permanent, c *Characteristics) {
			c.Power += 3
		},
	})
	gs.InvalidateCharacteristicsCache()
	if got := gs.PowerOf(p); got != 5 {
		t.Errorf("after invalidation power is %d, want 5 — stale value served", got)
	}
}
