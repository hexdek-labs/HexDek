package hexapi

import (
	"sync"
	"testing"
	"time"
)

// event_bus_test.go — unit tests for the in-process pub/sub primitive
// backing /ws/events. The bus is the load-bearing component; the
// websocket handler is mostly transport. Validate behaviour here
// without dragging in WebSocket wiring.

func TestEventBus_Subscribe_AddsToCount(t *testing.T) {
	b := NewEventBus()
	if got := b.SubscriberCount(); got != 0 {
		t.Fatalf("fresh bus: want 0 subscribers, got %d", got)
	}
	s := b.Subscribe(8)
	if got := b.SubscriberCount(); got != 1 {
		t.Errorf("after Subscribe: want 1, got %d", got)
	}
	b.Unsubscribe(s)
	if got := b.SubscriberCount(); got != 0 {
		t.Errorf("after Unsubscribe: want 0, got %d", got)
	}
}

func TestEventBus_Publish_DeliversToAllSubscribers(t *testing.T) {
	b := NewEventBus()
	s1 := b.Subscribe(8)
	s2 := b.Subscribe(8)
	t.Cleanup(func() {
		b.Unsubscribe(s1)
		b.Unsubscribe(s2)
	})

	n := b.Publish(Event{Type: EventGameEnd, Data: 42})
	if n != 2 {
		t.Fatalf("delivered count: want 2, got %d", n)
	}

	for i, s := range []*EventSubscriber{s1, s2} {
		select {
		case ev := <-s.Ch:
			if ev.Type != EventGameEnd {
				t.Errorf("s%d: event type: want %q, got %q", i, EventGameEnd, ev.Type)
			}
			if ev.Timestamp.IsZero() {
				t.Errorf("s%d: timestamp should be auto-populated", i)
			}
		case <-time.After(time.Second):
			t.Errorf("s%d: did not receive event within 1s", i)
		}
	}
}

func TestEventBus_Publish_PreservesExistingTimestamp(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(8)
	t.Cleanup(func() { b.Unsubscribe(s) })

	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	b.Publish(Event{Type: EventGameEnd, Timestamp: fixed})
	ev := <-s.Ch
	if !ev.Timestamp.Equal(fixed) {
		t.Errorf("timestamp: want %v, got %v", fixed, ev.Timestamp)
	}
}

func TestEventBus_Publish_Filter_OnlyDeliversMatchingTypes(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(8)
	t.Cleanup(func() { b.Unsubscribe(s) })
	s.SetFilter([]string{EventGameEnd})

	n := b.Publish(Event{Type: EventGameEnd, Data: "wanted"})
	if n != 1 {
		t.Errorf("matching event: delivered %d, want 1", n)
	}
	n = b.Publish(Event{Type: EventGauntletProgress, Data: "skipped"})
	if n != 0 {
		t.Errorf("non-matching event: delivered %d, want 0", n)
	}

	ev := <-s.Ch
	if ev.Type != EventGameEnd {
		t.Errorf("only the matching event should have landed: %+v", ev)
	}
	// Channel should now be empty.
	select {
	case ev := <-s.Ch:
		t.Errorf("unexpected second event: %+v", ev)
	default:
	}
}

func TestEventBus_Publish_EmptyFilter_DeliversEverything(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(8)
	t.Cleanup(func() { b.Unsubscribe(s) })
	s.SetFilter([]string{EventGameEnd})
	s.SetFilter(nil) // reset

	b.Publish(Event{Type: EventGameEnd})
	b.Publish(Event{Type: EventGauntletProgress})
	b.Publish(Event{Type: "custom.type"})

	got := drainChannel(t, s.Ch, 3, time.Second)
	if len(got) != 3 {
		t.Errorf("want 3 events with empty filter, got %d", len(got))
	}
}

func TestEventBus_Publish_FullBuffer_DropsRatherThanBlocks(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(2) // tiny buffer
	t.Cleanup(func() { b.Unsubscribe(s) })

	// Fill the buffer.
	b.Publish(Event{Type: EventGameEnd, Data: 1})
	b.Publish(Event{Type: EventGameEnd, Data: 2})

	// Third publish must NOT block — it drops.
	done := make(chan struct{})
	go func() {
		b.Publish(Event{Type: EventGameEnd, Data: 3})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("third Publish blocked on a full buffer — should drop instead")
	}

	if got := b.Drops(); got != 1 {
		t.Errorf("drops counter: want 1, got %d", got)
	}
}

func TestEventBus_Unsubscribe_ClosesChannel(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(4)
	b.Unsubscribe(s)
	// Reading a closed channel returns the zero value immediately.
	select {
	case _, ok := <-s.Ch:
		if ok {
			t.Errorf("channel should be closed after Unsubscribe")
		}
	case <-time.After(time.Second):
		t.Errorf("read on unsubscribed channel hung — channel was not closed")
	}
}

func TestEventBus_Unsubscribe_Idempotent(t *testing.T) {
	b := NewEventBus()
	s := b.Subscribe(4)
	b.Unsubscribe(s)
	// Second call must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("second Unsubscribe panicked: %v", r)
		}
	}()
	b.Unsubscribe(s)
}

func TestEventBus_Unsubscribe_NilSafe(t *testing.T) {
	b := NewEventBus()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Unsubscribe(nil) panicked: %v", r)
		}
	}()
	b.Unsubscribe(nil)
}

func TestEventBus_ConcurrentPublishersAndSubscribers(t *testing.T) {
	// Stress the mutexes: 4 publishers × 4 subscribers, 100
	// publishes each. The bus must not panic, must not deadlock,
	// and every subscriber must observe at least one event.
	b := NewEventBus()
	const subsCount = 4
	subs := make([]*EventSubscriber, subsCount)
	for i := range subs {
		subs[i] = b.Subscribe(2048)
	}
	defer func() {
		for _, s := range subs {
			b.Unsubscribe(s)
		}
	}()

	const pubCount = 4
	const eventsPerPub = 100
	var wg sync.WaitGroup
	for i := 0; i < pubCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < eventsPerPub; j++ {
				b.Publish(Event{Type: EventGameEnd})
			}
		}()
	}
	wg.Wait()

	for i, s := range subs {
		select {
		case <-s.Ch:
		case <-time.After(2 * time.Second):
			t.Errorf("subscriber %d observed nothing under concurrent publishers", i)
		}
	}
}

// drainChannel reads up to `want` events from ch within timeout,
// returning whatever it got. Helps tests assert on counts without
// hardcoding timing.
func drainChannel(t *testing.T, ch <-chan Event, want int, timeout time.Duration) []Event {
	t.Helper()
	out := make([]Event, 0, want)
	deadline := time.After(timeout)
	for len(out) < want {
		select {
		case ev, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, ev)
		case <-deadline:
			return out
		}
	}
	return out
}
