package gameengine

import (
	"math/rand"
	"reflect"
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

// r156 — the cache moved from a gs.charCache map onto the Permanent
// itself, which makes r73's bug structurally impossible rather than
// merely fixed: the GameState no longer holds a reference to the
// permanent at all, so a dead permanent's entry dies with it and there is
// nothing to clear.
//
// Counting map entries is therefore no longer the right assertion — the
// map is gone. This is the DERIVED GATE instead: no field on GameState
// may be a map keyed by *Permanent. That pins the property for good,
// deterministically, and will fail the day someone reintroduces the shape
// r73 was about.
func TestCharCacheR73_GameStateHoldsNoPerPermanentMap(t *testing.T) {
	ty := reflect.TypeOf(GameState{})
	permPtr := reflect.TypeOf((*Permanent)(nil))
	for i := 0; i < ty.NumField(); i++ {
		f := ty.Field(i)
		if f.Type.Kind() == reflect.Map && f.Type.Key() == permPtr {
			t.Errorf("GameState.%s is a map[*Permanent]%s — this is the r73 "+
				"retention shape: the map key keeps every permanent it ever "+
				"saw reachable for the life of the GameState",
				f.Name, f.Type.Elem())
		}
	}
}

// And the behaviour r73 actually cared about: churn a lot of permanents
// through the cache and none of them is left referenced by the GameState.
// With the entry on the permanent this is true by construction, so the
// test is cheap — it exists to fail loudly if the storage moves back.
func TestCharCacheR73_DeadPermanentsLeaveNothingBehind(t *testing.T) {
	gs := r73Game(t)
	const n = 500
	var last *Permanent
	for i := 0; i < n; i++ {
		p := addBattlefield(gs, 0, "Spirit Token", 1, 1, "creature")
		_ = GetEffectiveCharacteristics(gs, p) // caches an entry on p
		if p.charChars == nil {
			t.Fatalf("iteration %d: query did not populate the permanent's cache", i)
		}
		gs.Seats[0].Battlefield = gs.Seats[0].Battlefield[:0]
		gs.InvalidateCharacteristicsCache()
		last = p
	}
	// The last permanent still holds its own (now stale) entry — that is
	// fine and is the point: it is reachable only from the permanent, so
	// it is collected with it. What must NOT happen is the GameState
	// serving it.
	if last.charEpoch == gs.charCacheEpoch {
		t.Errorf("stale entry still considered current: charEpoch %d == epoch %d",
			last.charEpoch, gs.charCacheEpoch)
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
	if p.charChars == nil || p.charEpoch != gs.charCacheEpoch {
		t.Fatalf("query after invalidation did not populate the cache (chars=%v epoch=%d want %d)",
			p.charChars != nil, p.charEpoch, gs.charCacheEpoch)
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
