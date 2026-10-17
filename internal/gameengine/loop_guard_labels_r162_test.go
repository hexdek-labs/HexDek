package gameengine

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
)

// r162Fire drives a real LogLoopGuardFired with the details shape the
// production call site uses, under the policy production runs.
func r162Fire(t *testing.T, guard string, details map[string]interface{}) {
	t.Helper()
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.EventPolicy = EventLogNone
	LogLoopGuardFired(gs, guard, details)
}

func r162Reset(t *testing.T) {
	t.Helper()
	ResetLoopGuardCountsForTest()
	ResetLoopGuardLabelsForTest()
	t.Cleanup(func() {
		ResetLoopGuardCountsForTest()
		ResetLoopGuardLabelsForTest()
	})
}

// The premise of the round, pinned as its own test: under the policy
// production runs, the details map — and therefore the event name — is
// dropped entirely. If this ever stops being true the round is solving
// a problem that no longer exists.
func TestR162_EventLogNoneStillDropsTheDetails(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	gs.EventPolicy = EventLogNone
	LogLoopGuardFired(gs, "percard_dispatch_depth_cap",
		map[string]interface{}{"depth": 9, "event": "creature_etb"})
	if len(gs.EventLog) != 0 {
		t.Fatalf("EventLogNone retained %d events; the premise of r160/r162 "+
			"is that it retains nothing", len(gs.EventLog))
	}
}

// ...and the attribution survives it.
func TestR162_TheEventNameSurvivesTheDroppedEvent(t *testing.T) {
	r162Reset(t)
	for i := 0; i < 3; i++ {
		r162Fire(t, "percard_dispatch_depth_cap",
			map[string]interface{}{"depth": 9, "event": "creature_etb"})
	}
	r162Fire(t, "percard_dispatch_depth_cap",
		map[string]interface{}{"depth": 9, "event": "permanent_ltb"})

	got := LoopGuardLabelCounts()
	if got["percard_dispatch_depth_cap/creature_etb"] != 3 {
		t.Fatalf("creature_etb counted %d, want 3 (%v)",
			got["percard_dispatch_depth_cap/creature_etb"], got)
	}
	if got["percard_dispatch_depth_cap/permanent_ltb"] != 1 {
		t.Fatalf("permanent_ltb counted %d, want 1 (%v)",
			got["percard_dispatch_depth_cap/permanent_ltb"], got)
	}
	// r160's per-guard total must be unchanged by the new accounting.
	if n := LoopGuardCounts()["percard_dispatch_depth_cap"]; n != 4 {
		t.Fatalf("per-guard total is %d, want 4 — guards= must not move", n)
	}
}

// "handler" is the other allow-listed key (move_no_progress uses it).
func TestR162_HandlerIsAttributedToo(t *testing.T) {
	r162Reset(t)
	r162Fire(t, "move_no_progress", map[string]interface{}{"handler": "chaos_wand"})
	if LoopGuardLabelCounts()["move_no_progress/chaos_wand"] != 1 {
		t.Fatalf("handler not attributed: %v", LoopGuardLabelCounts())
	}
}

// A guard whose details carry no allow-listed key is counted by r160
// and simply not attributed — never attributed to a junk label.
func TestR162_UnlabelledGuardIsCountedButNotAttributed(t *testing.T) {
	r162Reset(t)
	r162Fire(t, "sba_max_passes", map[string]interface{}{"passes": 60})
	if n := LoopGuardCounts()["sba_max_passes"]; n != 1 {
		t.Fatalf("per-guard count is %d, want 1", n)
	}
	if got := LoopGuardLabelCounts(); len(got) != 0 {
		t.Fatalf("unlabelled guard produced attribution %v; a numeric detail "+
			"must never become a label", got)
	}
}

// The cardinality bound. Past loopGuardLabelMax distinct labels, new
// ones collapse into the overflow bucket rather than growing the map.
func TestR162_CardinalityIsBounded(t *testing.T) {
	r162Reset(t)
	const extra = 40
	for i := 0; i < loopGuardLabelMax+extra; i++ {
		r162Fire(t, "percard_dispatch_depth_cap",
			map[string]interface{}{"event": fmt.Sprintf("evt_%03d", i)})
	}
	got := LoopGuardLabelCounts()
	if len(got) > loopGuardLabelMax+1 {
		t.Fatalf("label map grew to %d entries, bound is %d (+1 overflow)",
			len(got), loopGuardLabelMax)
	}
	if got["percard_dispatch_depth_cap/"+loopGuardLabelOverflow] != extra {
		t.Fatalf("overflow bucket holds %d, want %d (%d entries total)",
			got["percard_dispatch_depth_cap/"+loopGuardLabelOverflow], extra, len(got))
	}
	// Nothing was lost: every firing is still counted somewhere.
	var sum uint64
	for _, v := range got {
		sum += v
	}
	if sum != uint64(loopGuardLabelMax+extra) {
		t.Fatalf("attributed %d firings, fired %d", sum, loopGuardLabelMax+extra)
	}
}

// The summary is heaviest-first and stable, because it is read by a
// human scanning a log line.
func TestR162_TopIsSortedByCountThenName(t *testing.T) {
	r162Reset(t)
	fire := func(event string, n int) {
		for i := 0; i < n; i++ {
			r162Fire(t, "g", map[string]interface{}{"event": event})
		}
	}
	fire("bbb", 5)
	fire("aaa", 5) // ties with bbb — name breaks it
	fire("ccc", 9)
	fire("ddd", 1)

	want := "g/ccc:9,g/aaa:5,g/bbb:5"
	if got := LoopGuardLabelTop(3); got != want {
		t.Fatalf("top(3) = %q, want %q", got, want)
	}
	if got := LoopGuardLabelTop(0); got != "" {
		t.Fatalf("top(0) = %q, want empty", got)
	}
	if got := LoopGuardLabelTop(99); strings.Count(got, ",") != 3 {
		t.Fatalf("top(99) = %q, want all four pairs", got)
	}
}

// Nothing fired -> empty, so the grinder line gains no permanent column.
func TestR162_EmptyWhenNothingFired(t *testing.T) {
	r162Reset(t)
	if got := LoopGuardLabelTop(5); got != "" {
		t.Fatalf("top = %q with no firings, want empty", got)
	}
}

// Two workers fire concurrently in production; run this under -race.
func TestR162_ConcurrentFiringIsSafe(t *testing.T) {
	r162Reset(t)
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				gs := NewGameState(2, rand.New(rand.NewSource(int64(w))), nil)
				gs.EventPolicy = EventLogNone
				LogLoopGuardFired(gs, "percard_dispatch_depth_cap",
					map[string]interface{}{"event": fmt.Sprintf("e%d", i%4)})
			}
		}(w)
	}
	wg.Wait()
	var sum uint64
	for _, v := range LoopGuardLabelCounts() {
		sum += v
	}
	if sum != 400 {
		t.Fatalf("attributed %d firings, fired 400", sum)
	}
}
