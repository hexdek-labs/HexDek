package gameengine

import (
	"strings"
	"sync"
	"testing"
)

// r160 — loop-guard firings must survive EventLogNone, which is the
// policy production actually runs. The event they emit is dropped
// there ("Retain nothing"), so for nine guards the grinder had no
// record at all.

// r160Fire fires a guard against a GameState with the given policy.
func r160Fire(t *testing.T, policy EventLogPolicy, guard string) *GameState {
	t.Helper()
	gs := newFixtureGame(t)
	gs.EventPolicy = policy
	LogLoopGuardFired(gs, guard, map[string]interface{}{"site": "r160"})
	return gs
}

// THE HEADLINE, direction 1: under EventLogNone the event is dropped —
// this pins the behaviour the counter exists to work around, so the
// test states the problem rather than assuming it.
func TestR160_EventLogNoneDropsTheGuardEvent(t *testing.T) {
	ResetLoopGuardCountsForTest()
	gs := r160Fire(t, EventLogNone, "r160_dropped")

	for _, ev := range gs.EventLog {
		if ev.Kind == EventLoopGuardFired {
			t.Fatalf("precondition failed: EventLogNone retained the event, so " +
				"this round is solving a problem that does not exist")
		}
	}
}

// Direction 2: the counter records it anyway.
func TestR160_CounterSurvivesEventLogNone(t *testing.T) {
	ResetLoopGuardCountsForTest()
	r160Fire(t, EventLogNone, "r160_counted")

	got := LoopGuardCounts()
	if got["r160_counted"] != 1 {
		t.Errorf("counter = %d, want 1 — the firing left no record under the "+
			"policy production runs", got["r160_counted"])
	}
}

// A guard that never fires must be ABSENT, not zero, so the grinder
// line stays unchanged in the normal case.
func TestR160_UnfiredGuardsAreAbsentAndSummaryIsEmpty(t *testing.T) {
	ResetLoopGuardCountsForTest()

	if s := LoopGuardSummary(); s != "" {
		t.Errorf("summary with nothing fired = %q, want empty so the window "+
			"line gains no permanent noise column", s)
	}
	if _, ok := LoopGuardCounts()["never_fired_r160"]; ok {
		t.Errorf("an unfired guard appeared in the snapshot")
	}
}

// The summary must be sorted and stable, or the window line churns.
func TestR160_SummaryIsSortedAndStable(t *testing.T) {
	ResetLoopGuardCountsForTest()
	for _, g := range []string{"zeta_r160", "alpha_r160", "mid_r160"} {
		r160Fire(t, EventLogNone, g)
	}
	r160Fire(t, EventLogNone, "alpha_r160") // alpha fires twice

	first := LoopGuardSummary()
	if first != LoopGuardSummary() {
		t.Errorf("summary is not stable across calls")
	}
	if !strings.Contains(first, "alpha_r160:2") {
		t.Errorf("summary %q does not carry alpha_r160:2", first)
	}
	ia := strings.Index(first, "alpha_r160")
	im := strings.Index(first, "mid_r160")
	iz := strings.Index(first, "zeta_r160")
	if !(ia < im && im < iz) {
		t.Errorf("summary is not sorted by name: %q", first)
	}
}

// A guard fired with a nil GameState still fired. The counter bump is
// deliberately outside the nil guard.
func TestR160_NilGameStateStillCounts(t *testing.T) {
	ResetLoopGuardCountsForTest()
	LogLoopGuardFired(nil, "r160_nil", nil)

	if LoopGuardCounts()["r160_nil"] != 1 {
		t.Errorf("a guard that fired without a GameState was not counted")
	}
}

// Two grinder workers fire guards concurrently. Run with -race.
func TestR160_ConcurrentFiringIsRaceFree(t *testing.T) {
	ResetLoopGuardCountsForTest()

	const workers, each = 4, 200
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				LogLoopGuardFired(nil, "r160_concurrent", nil)
			}
		}()
	}
	wg.Wait()

	if got := LoopGuardCounts()["r160_concurrent"]; got != workers*each {
		t.Errorf("counter = %d, want %d — a firing was lost", got, workers*each)
	}
}
