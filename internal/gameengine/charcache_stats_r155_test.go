package gameengine

import (
	"sync"
	"testing"
)

func r155Perm() *Permanent {
	return &Permanent{Card: &Card{Name: "Body", Types: []string{"creature"}}, Flags: map[string]int{}}
}

// A cold call is a miss; a repeat is a hit; an invalidation makes the next
// one a miss again. If any of these is wrong the production hit rate the
// round exists to read is wrong.
func TestCharCacheR155_CountsHitsMissesAndInvalidations(t *testing.T) {
	gs := newFixtureGame(t)
	p := r155Perm()

	GetEffectiveCharacteristics(gs, p) // miss
	if gs.ccMisses != 1 || gs.ccHits != 0 {
		t.Fatalf("cold call: want 1 miss 0 hits, got %d/%d", gs.ccMisses, gs.ccHits)
	}

	GetEffectiveCharacteristics(gs, p) // hit
	GetEffectiveCharacteristics(gs, p) // hit
	if gs.ccMisses != 1 || gs.ccHits != 2 {
		t.Fatalf("warm calls: want 1 miss 2 hits, got %d/%d", gs.ccMisses, gs.ccHits)
	}

	before := gs.ccInval
	gs.InvalidateCharacteristicsCache()
	if gs.ccInval != before+1 {
		t.Fatalf("invalidation not counted: %d -> %d", before, gs.ccInval)
	}

	GetEffectiveCharacteristics(gs, p) // miss again
	if gs.ccMisses != 2 {
		t.Fatalf("post-invalidation: want 2 misses, got %d", gs.ccMisses)
	}

	// The rate the log line will print, computed the same way.
	rate := float64(gs.ccHits) / float64(gs.ccHits+gs.ccMisses) * 100
	if rate < 49 || rate > 51 {
		t.Fatalf("2 hits / 4 calls should read ~50%%, got %.1f", rate)
	}
}

// CONTROL — counting a hit unconditionally (the obvious off-by-one) reports
// 100% and would have told us the cache was fine. It must not match what
// the real counters produce.
func TestCharCacheR155_ControlUnconditionalHitReads100(t *testing.T) {
	gs := newFixtureGame(t)
	p := r155Perm()

	calls := 4
	ctlHits := 0
	for i := 0; i < calls; i++ {
		if i == 2 {
			gs.InvalidateCharacteristicsCache()
		}
		GetEffectiveCharacteristics(gs, p)
		ctlHits++ // the broken version
	}
	ctlRate := float64(ctlHits) / float64(calls) * 100
	if ctlRate != 100 {
		t.Fatalf("CONTROL DID NOT GO RED: the broken counter should read 100%%, got %.1f", ctlRate)
	}
	realRate := float64(gs.ccHits) / float64(gs.ccHits+gs.ccMisses) * 100
	if realRate >= 100 {
		t.Fatalf("the real counter matched the broken one at %.1f%% — it is not measuring misses", realRate)
	}
	t.Logf("broken counter reads %.0f%%, real counter reads %.1f%%", ctlRate, realRate)
}

// Publishing folds into the process-wide totals and resets the game's own.
func TestCharCacheR155_PublishFoldsAndResets(t *testing.T) {
	ResetCharCacheCountersForTest()
	gs := newFixtureGame(t)
	p := r155Perm()

	GetEffectiveCharacteristics(gs, p)
	GetEffectiveCharacteristics(gs, p)
	gs.InvalidateCharacteristicsCache()

	wantH, wantM, wantI := gs.ccHits, gs.ccMisses, gs.ccInval
	if wantH == 0 || wantM == 0 || wantI == 0 {
		t.Fatalf("precondition: all three counters should be non-zero, got %d/%d/%d", wantH, wantM, wantI)
	}

	gs.PublishCharCacheStats()

	h, m, i := CharCacheCounters()
	if h != wantH || m != wantM || i != wantI {
		t.Fatalf("publish lost counts: want %d/%d/%d, got %d/%d/%d", wantH, wantM, wantI, h, m, i)
	}
	if gs.ccHits != 0 || gs.ccMisses != 0 || gs.ccInval != 0 {
		t.Fatalf("publish did not reset the game's counters: %d/%d/%d", gs.ccHits, gs.ccMisses, gs.ccInval)
	}

	// A second publish must not double-count.
	gs.PublishCharCacheStats()
	h2, m2, i2 := CharCacheCounters()
	if h2 != h || m2 != m || i2 != i {
		t.Fatalf("double publish double-counted: %d/%d/%d -> %d/%d/%d", h, m, i, h2, m2, i2)
	}
}

// The grinder runs two workers. Publishing must be race-free and must not
// lose counts. Run with -race.
func TestCharCacheR155_ConcurrentPublishIsRaceFreeAndExact(t *testing.T) {
	ResetCharCacheCountersForTest()

	const workers, perWorker = 8, 50
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for g := 0; g < perWorker; g++ {
				gs := &GameState{}
				p := r155Perm()
				GetEffectiveCharacteristics(gs, p) // miss
				GetEffectiveCharacteristics(gs, p) // hit
				gs.PublishCharCacheStats()
			}
		}()
	}
	wg.Wait()

	h, m, _ := CharCacheCounters()
	want := uint64(workers * perWorker)
	if h != want || m != want {
		t.Fatalf("counts lost under concurrency: want %d hits %d misses, got %d/%d", want, want, h, m)
	}
}
