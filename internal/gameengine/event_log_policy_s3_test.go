package gameengine

import (
	"fmt"
	"math/rand"
	"testing"
)

// event_log_policy_s3_test.go — consolidation step 3: the EventLog +
// lastEvent dual path collapsed into one policy-governed log
// {full|ring|none}. Pins each policy's retention contract, the
// zero-value-is-full-retention guarantee (the goldilocks 1,795
// keyword_dead bug was a fixture silently dropping events by omission),
// and that Hats observe every event under every policy.

func logN(gs *GameState, n int) {
	for i := 0; i < n; i++ {
		gs.LogEvent(Event{Kind: "probe", Amount: i})
	}
}

func TestEventPolicy_FullRetainsEverything(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(1)), nil)
	base := len(gs.EventLog)
	logN(gs, 1000)
	if len(gs.EventLog) != base+1000 {
		t.Fatalf("full policy retained %d, want %d", len(gs.EventLog)-base, 1000)
	}
	if gs.EventLog[len(gs.EventLog)-1].Amount != 999 {
		t.Fatalf("last retained event = %+v, want Amount=999", gs.EventLog[len(gs.EventLog)-1])
	}
}

func TestEventPolicy_FullCapStopsAppending(t *testing.T) {
	gs := &GameState{EventPolicy: EventLogFull}
	gs.EventLog = make([]Event, maxEventLog) // pre-filled to the cap
	logN(gs, 5)
	if len(gs.EventLog) != maxEventLog {
		t.Fatalf("over-cap append grew the log to %d", len(gs.EventLog))
	}
	if gs.EventsLogged != 5 {
		t.Fatalf("EventsLogged = %d, want 5 (counts regardless of retention)", gs.EventsLogged)
	}
}

// TestEventPolicy_ZeroValueRetains is the goldilocks pin: a
// struct-literal GameState that never sets EventPolicy must RETAIN
// events. Pre-step-3, the zero value of RetainEvents (false) silently
// dropped every event to the single-slot lastEvent, blinding any
// event-log probe (1,795 keyword_dead misreports).
func TestEventPolicy_ZeroValueRetains(t *testing.T) {
	gs := &GameState{} // fixture that forgot to configure logging
	logN(gs, 10)
	if len(gs.EventLog) != 10 {
		t.Fatalf("zero-value policy retained %d events, want 10 — the goldilocks silent-drop footgun is back", len(gs.EventLog))
	}
}

func TestEventPolicy_NoneRetainsNothing(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(2)), nil)
	gs.EventPolicy = EventLogNone
	base := len(gs.EventLog)
	logN(gs, 100)
	if len(gs.EventLog) != base {
		t.Fatalf("none policy retained %d new events", len(gs.EventLog)-base)
	}
	if gs.EventsLogged < 100 {
		t.Fatalf("EventsLogged = %d, want >= 100 (work-volume proxy counts under every policy)", gs.EventsLogged)
	}
}

func TestEventPolicy_RingBoundsAndOrder(t *testing.T) {
	const ring = 64
	gs := &GameState{EventPolicy: EventLogRing, EventRingSize: ring}
	logN(gs, 10*ring)

	if len(gs.EventLog) < ring || len(gs.EventLog) > 2*ring {
		t.Fatalf("ring retained %d events, want within [%d, %d]", len(gs.EventLog), ring, 2*ring)
	}
	// Chronological order, ending at the newest event.
	for i := 1; i < len(gs.EventLog); i++ {
		if gs.EventLog[i].Amount != gs.EventLog[i-1].Amount+1 {
			t.Fatalf("ring order broken at %d: %d after %d", i, gs.EventLog[i].Amount, gs.EventLog[i-1].Amount)
		}
	}
	if last := gs.EventLog[len(gs.EventLog)-1].Amount; last != 10*ring-1 {
		t.Fatalf("ring's newest event Amount = %d, want %d", last, 10*ring-1)
	}
	// RecentEvents gives an exact-width tail regardless of compaction phase.
	tail := gs.RecentEvents(ring)
	if len(tail) != ring || tail[len(tail)-1].Amount != 10*ring-1 {
		t.Fatalf("RecentEvents(%d): len=%d last=%d", ring, len(tail), tail[len(tail)-1].Amount)
	}
}

func TestEventPolicy_RingDefaultSize(t *testing.T) {
	gs := &GameState{EventPolicy: EventLogRing} // EventRingSize unset
	logN(gs, 3*defaultEventRingSize)
	if len(gs.EventLog) < defaultEventRingSize || len(gs.EventLog) > 2*defaultEventRingSize {
		t.Fatalf("default ring retained %d, want within [%d, %d]",
			len(gs.EventLog), defaultEventRingSize, 2*defaultEventRingSize)
	}
}

// observingHat records every event Kind it sees; used to pin the
// broadcast-under-every-policy contract that lastEvent used to serve.
type observingHat struct {
	GreedyHatStub // triggers_test.go's no-op Hat implementation
	seen          []string
}

func (h *observingHat) ObserveEvent(gs *GameState, seatIdx int, ev *Event) {
	if ev != nil {
		h.seen = append(h.seen, ev.Kind)
	}
}

func TestEventPolicy_HatsObserveUnderEveryPolicy(t *testing.T) {
	for _, policy := range []EventLogPolicy{EventLogFull, EventLogRing, EventLogNone, ""} {
		t.Run(string(policy)+"_policy", func(t *testing.T) {
			gs := NewGameState(2, rand.New(rand.NewSource(3)), nil)
			gs.EventPolicy = policy
			hat := &observingHat{}
			gs.Seats[0].Hat = hat
			for i := 0; i < 5; i++ {
				gs.LogEvent(Event{Kind: fmt.Sprintf("probe_%d", i)})
			}
			if len(hat.seen) != 5 {
				t.Fatalf("hat observed %d events under policy %q, want 5", len(hat.seen), policy)
			}
			if hat.seen[4] != "probe_4" {
				t.Fatalf("hat saw %q last, want probe_4", hat.seen[4])
			}
		})
	}
}

// TestEventPolicy_CloneIsNone — rollout clones must not start
// accumulating events now that the zero value means full retention
// (pre-step-3 they inherited RetainEvents=false implicitly).
func TestEventPolicy_CloneIsNone(t *testing.T) {
	gs := NewGameState(2, rand.New(rand.NewSource(4)), nil)
	clone := gs.CloneForRollout(rand.New(rand.NewSource(5)))
	if clone.EventPolicy != EventLogNone {
		t.Fatalf("clone EventPolicy = %q, want none (rollouts are allocation-free)", clone.EventPolicy)
	}
	before := len(clone.EventLog)
	clone.LogEvent(Event{Kind: "rollout_probe"})
	if len(clone.EventLog) != before {
		t.Fatalf("rollout clone retained a new event")
	}
}
