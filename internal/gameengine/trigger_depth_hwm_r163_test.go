package gameengine

import (
	"math/rand"
	"sync"
	"testing"
)

func r163Game(t *testing.T) *GameState {
	t.Helper()
	return NewGameState(2, rand.New(rand.NewSource(1)), nil)
}

func r163Reset(t *testing.T) {
	t.Helper()
	ResetTriggerDepthForTest()
	t.Cleanup(ResetTriggerDepthForTest)
}

// The published number is a SURVIVAL CURVE: games reaching at least
// depth d. A game climbing 1..5 must contribute one to every level it
// passed through, or the curve cannot be read as "how deep do games
// get".
func TestR163_ClimbingRecordsEveryLevelPassed(t *testing.T) {
	r163Reset(t)
	gs := r163Game(t)
	for d := 1; d <= 5; d++ {
		gs.NoteTriggerDepth(d)
	}
	h := TriggerDepthHistogram()
	for d := 1; d <= 5; d++ {
		if h[d] != 1 {
			t.Fatalf("depth %d counted %d, want 1 (%v)", d, h[d], h)
		}
	}
	if _, ok := h[6]; ok {
		t.Fatalf("depth 6 recorded but never reached (%v)", h)
	}
}

// Unwinding and re-climbing within ONE game must not double-count: the
// high-water mark is the point. Otherwise a game with many shallow
// cascades looks like a deep one.
func TestR163_ReClimbingTheSameGameDoesNotDoubleCount(t *testing.T) {
	r163Reset(t)
	gs := r163Game(t)
	for rep := 0; rep < 10; rep++ {
		for d := 1; d <= 4; d++ {
			gs.NoteTriggerDepth(d)
		}
	}
	h := TriggerDepthHistogram()
	for d := 1; d <= 4; d++ {
		if h[d] != 1 {
			t.Fatalf("depth %d counted %d after 10 identical cascades, want 1 "+
				"— this is a high-water mark, not a visit count (%v)", d, h[d], h)
		}
	}
}

// Separate games each contribute.
func TestR163_SeparateGamesEachCount(t *testing.T) {
	r163Reset(t)
	for g := 0; g < 3; g++ {
		gs := r163Game(t)
		gs.NoteTriggerDepth(1)
		gs.NoteTriggerDepth(2)
	}
	h := TriggerDepthHistogram()
	if h[1] != 3 || h[2] != 3 {
		t.Fatalf("want 3 games at depths 1 and 2, got %v", h)
	}
}

// The curve must be monotonically non-increasing with depth — that is
// what makes it a survival curve and what makes "87% of games reach 8"
// a meaningful sentence.
func TestR163_CurveIsMonotonic(t *testing.T) {
	r163Reset(t)
	for g, peak := range []int{2, 5, 5, 9, 1} {
		_ = g
		gs := r163Game(t)
		for d := 1; d <= peak; d++ {
			gs.NoteTriggerDepth(d)
		}
	}
	h := TriggerDepthHistogram()
	prev := uint64(1 << 62)
	for d := 1; d <= 9; d++ {
		if h[d] > prev {
			t.Fatalf("curve rose at depth %d (%d > %d): not a survival curve (%v)",
				d, h[d], prev, h)
		}
		prev = h[d]
	}
	if h[1] != 5 || h[9] != 1 {
		t.Fatalf("want 5 games at depth 1 and 1 at depth 9, got %v", h)
	}
}

// Defensive bound: the fireTrigger cap is 8 today, but the histogram
// must not become an unbounded array if that constant ever moves.
func TestR163_DepthIsBounded(t *testing.T) {
	r163Reset(t)
	gs := r163Game(t)
	gs.NoteTriggerDepth(triggerDepthMax + 500)
	h := TriggerDepthHistogram()
	if h[triggerDepthMax] != 1 {
		t.Fatalf("an out-of-range depth must land in the top bucket, got %v", h)
	}
	if len(h) != 1 {
		t.Fatalf("want exactly one bucket, got %v", h)
	}
}

func TestR163_SummaryIsShallowestFirstAndEmptyWhenUnused(t *testing.T) {
	r163Reset(t)
	if got := TriggerDepthSummary(); got != "" {
		t.Fatalf("summary = %q with nothing recorded, want empty", got)
	}
	gs := r163Game(t)
	for d := 1; d <= 3; d++ {
		gs.NoteTriggerDepth(d)
	}
	if got, want := TriggerDepthSummary(), "1:1,2:1,3:1"; got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func TestR163_NilAndZeroAreSafe(t *testing.T) {
	r163Reset(t)
	var gs *GameState
	gs.NoteTriggerDepth(3)
	r163Game(t).NoteTriggerDepth(0)
	r163Game(t).NoteTriggerDepth(-1)
	if got := TriggerDepthHistogram(); len(got) != 0 {
		t.Fatalf("nil/zero/negative recorded something: %v", got)
	}
}

// Two grinder workers run concurrently; run under -race.
func TestR163_ConcurrentGamesAreSafe(t *testing.T) {
	r163Reset(t)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
				for d := 1; d <= 4; d++ {
					gs.NoteTriggerDepth(d)
				}
			}
		}()
	}
	wg.Wait()
	h := TriggerDepthHistogram()
	for d := 1; d <= 4; d++ {
		if h[d] != 200 {
			t.Fatalf("depth %d counted %d, want 200", d, h[d])
		}
	}
}
