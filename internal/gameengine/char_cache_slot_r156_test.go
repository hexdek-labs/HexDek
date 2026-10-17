package gameengine

import (
	"math/rand"
	"testing"
)

func r156Game() *GameState {
	return NewGameState(2, rand.New(rand.NewSource(11)), nil)
}

// r156Anthem gives p +3/+3 in gs only.
func r156Anthem(gs *GameState, p *Permanent) {
	gs.RegisterContinuousEffect(&ContinuousEffect{
		Layer: LayerPT, Sublayer: "c", HandlerID: "r156_anthem",
		Duration:  DurationPermanent,
		Predicate: func(_ *GameState, q *Permanent) bool { return q == p },
		ApplyFn:   func(_ *GameState, _ *Permanent, c *Characteristics) { c.Power += 3 },
	})
}

// THE LANDMINE. Two GameStates at the SAME epoch holding the same
// permanent must not serve each other's characteristics. Before the
// cacheID guard this returned 5 for both — I confirmed it by probe.
//
// Nothing shares a permanent today (Clone deep-copies through a field
// literal and remaps pointers), so this test is insurance against a
// future path that does. The failure it guards is silent: a wrong power,
// type or keyword with no error anywhere.
func TestCharCacheR156_TwoGameStatesDoNotShareAnEntry(t *testing.T) {
	gsA, gsB := r156Game(), r156Game()
	p := addBattlefield(gsA, 0, "Bear", 2, 2, "creature")
	r156Anthem(gsA, p)

	// Force the epochs equal — that is the colliding case.
	for gsB.charCacheEpoch < gsA.charCacheEpoch {
		gsB.InvalidateCharacteristicsCache()
	}
	if gsA.charCacheEpoch != gsB.charCacheEpoch {
		t.Fatalf("precondition: epochs should match, got %d vs %d",
			gsA.charCacheEpoch, gsB.charCacheEpoch)
	}

	if got := gsA.PowerOf(p); got != 5 {
		t.Fatalf("gsA sees its own anthem: want 5, got %d", got)
	}
	if got := gsB.PowerOf(p); got != 2 {
		t.Fatalf("gsB served gsA's cached entry: want 2, got %d", got)
	}
	// And the other direction, now that gsB has stashed its own.
	if got := gsA.PowerOf(p); got != 5 {
		t.Fatalf("gsA lost its entry to gsB: want 5, got %d", got)
	}
}

// A fresh GameState is at epoch 0 and a fresh Permanent is at charEpoch 0,
// so the epoch alone compares equal on a cold read. What rejects it is
// charGS, not the nil check — I had that backwards until a control that
// dropped the nil check stayed green.
func TestCharCacheR156_ColdReadAtEpochZeroIsAMiss(t *testing.T) {
	gs := &GameState{} // deliberately a bare literal, epoch 0, cacheID 0
	p := &Permanent{Card: &Card{Name: "Bear", Types: []string{"creature"}}, Flags: map[string]int{}}

	if gs.charCacheEpoch != 0 || p.charEpoch != 0 {
		t.Fatalf("precondition: both should be zero, got %d / %d", gs.charCacheEpoch, p.charEpoch)
	}
	chars := GetEffectiveCharacteristics(gs, p)
	if chars == nil {
		t.Fatal("cold read at epoch 0 returned nil — the nil guard is gone")
	}
	if gs.ccMisses != 1 || gs.ccHits != 0 {
		t.Fatalf("cold read should be a miss: hits=%d misses=%d", gs.ccHits, gs.ccMisses)
	}
}

// Two bare &GameState{} literals must get distinct identities, or they
// collide exactly as two zero values would.
func TestCharCacheR156_BareLiteralsGetDistinctIdentities(t *testing.T) {
	gsA, gsB := &GameState{}, &GameState{}
	p := &Permanent{Card: &Card{Name: "Bear", Types: []string{"creature"}}, Flags: map[string]int{}}

	GetEffectiveCharacteristics(gsA, p)
	GetEffectiveCharacteristics(gsB, p)

	if gsA.cacheID == 0 || gsB.cacheID == 0 {
		t.Fatalf("cacheID not assigned: %d / %d", gsA.cacheID, gsB.cacheID)
	}
	if gsA.cacheID == gsB.cacheID {
		t.Fatalf("two GameStates share cacheID %d", gsA.cacheID)
	}
	// gsB's query must have been a miss, not a hit on gsA's entry.
	if gsB.ccHits != 0 {
		t.Fatalf("gsB hit on gsA's entry: hits=%d", gsB.ccHits)
	}
}

// The behaviour the cache exists for, unchanged: repeat queries under one
// epoch are served from the slot, and an invalidation recomputes.
func TestCharCacheR156_CachesWithinEpochAndInvalidates(t *testing.T) {
	gs := r156Game()
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")

	first := GetEffectiveCharacteristics(gs, p)
	second := GetEffectiveCharacteristics(gs, p)
	if first != second {
		t.Error("repeat query under one epoch was not served from the slot")
	}

	r156Anthem(gs, p) // registering bumps the epoch
	if got := gs.PowerOf(p); got != 5 {
		t.Errorf("stale value served after invalidation: want 5, got %d", got)
	}
}

// The hit path must not allocate — it is a field read now, and it runs
// ~430,000 times per game.
func TestCharCacheR156_HitPathAllocatesNothing(t *testing.T) {
	gs := r156Game()
	p := addBattlefield(gs, 0, "Bear", 2, 2, "creature")
	GetEffectiveCharacteristics(gs, p) // warm

	got := testing.AllocsPerRun(100, func() {
		GetEffectiveCharacteristics(gs, p)
	})
	if got != 0 {
		t.Fatalf("hit path allocated %v times, want 0", got)
	}
}
