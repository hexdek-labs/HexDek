package hexapi

import (
	"sync"
	"time"
)

// event_bus.go — in-process pub/sub that backs the /ws/events
// WebSocket stream. The engine loop publishes typed events
// (game.end, game.start, gauntlet.progress, ...) to the bus; every
// connected subscriber receives the ones they've opted into.
//
// Design constraints:
//
//   - Non-blocking publish. The engine loop must NEVER stall on a
//     slow consumer. Each subscriber has a bounded outgoing channel;
//     when it fills, Publish drops the message for that subscriber
//     (logged so the subscriber learns it was lossy on reconnect).
//
//   - Per-subscriber event filter. Clients subscribe to a set of
//     event-type strings; an empty set means "all events". Filter
//     mutation is mutex-guarded so the websocket reader can update
//     it concurrent with Publish.
//
//   - Stateless wrt the engine. The bus does NOT remember past
//     events — clients that disconnect mid-stream lose anything
//     emitted while they were away. The webhook system covers the
//     durable / replayable side of the event surface.

// Engine event-type constants. New events go here so callers can
// reference them without typos. The wire-format keeps them as plain
// strings — no enum gating — to leave room for future event types
// the bus doesn't yet know about.
const (
	EventGameEnd          = "game.end"
	EventGauntletProgress = "gauntlet.progress"
	EventELOUpdate        = "elo.update"
)

// Event is the wire shape of one published message. The bus emits
// this directly to the WebSocket writer — JSON-encoded — so the
// field names mirror the consumer's expected JSON.
type Event struct {
	Type      string    `json:"event"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

// EventBus is the central pub/sub hub. The zero value is NOT ready
// to use — call NewEventBus to wire the subscriber map. Multiple
// publishers and subscribers run concurrently; the bus uses an
// RWMutex over its subscriber set for low publish contention.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[*EventSubscriber]struct{}
	drops       int64 // process-lifetime count of dropped events (debug)
}

// EventSubscriber is one consumer's view of the bus. Outgoing
// messages arrive on Ch; the consumer reads until the channel is
// closed (which happens on Unsubscribe). Filter lets the consumer
// narrow to specific event types — nil / empty = all events.
type EventSubscriber struct {
	bus    *EventBus
	Ch     chan Event
	mu     sync.Mutex
	filter map[string]bool
}

// NewEventBus returns an EventBus ready for Subscribe / Publish.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[*EventSubscriber]struct{}),
	}
}

// Subscribe registers a new consumer. The returned channel buffers
// up to `buffer` events; when full, Publish drops further messages
// for this subscriber rather than blocking. 32 is a reasonable
// default for a single connected client.
func (b *EventBus) Subscribe(buffer int) *EventSubscriber {
	if buffer <= 0 {
		buffer = 32
	}
	s := &EventSubscriber{
		bus:    b,
		Ch:     make(chan Event, buffer),
		filter: map[string]bool{},
	}
	b.mu.Lock()
	b.subscribers[s] = struct{}{}
	b.mu.Unlock()
	return s
}

// Unsubscribe removes the consumer and closes its channel. Safe to
// call from any goroutine; idempotent — a second call is a no-op.
func (b *EventBus) Unsubscribe(s *EventSubscriber) {
	if s == nil {
		return
	}
	b.mu.Lock()
	_, ok := b.subscribers[s]
	if ok {
		delete(b.subscribers, s)
	}
	b.mu.Unlock()
	if ok {
		close(s.Ch)
	}
}

// SetFilter replaces the subscriber's event-type allowlist. An empty
// or nil events list means "all events" — the most common shape
// since /ws/events defaults to subscribe-to-all on connect.
func (s *EventSubscriber) SetFilter(events []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(events) == 0 {
		s.filter = map[string]bool{}
		return
	}
	s.filter = make(map[string]bool, len(events))
	for _, e := range events {
		s.filter[e] = true
	}
}

// wants returns whether the subscriber's filter accepts eventType.
// An empty filter accepts everything.
func (s *EventSubscriber) wants(eventType string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.filter) == 0 {
		return true
	}
	return s.filter[eventType]
}

// Publish fans an event out to every matching subscriber. Each send
// is a non-blocking select; if the consumer's buffer is full the
// event is dropped for that consumer (and the bus-level drop
// counter ticks). Returns the count of subscribers the event was
// actually delivered to — handy for tests + observability.
func (b *EventBus) Publish(e Event) int {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	b.mu.RLock()
	subs := make([]*EventSubscriber, 0, len(b.subscribers))
	for s := range b.subscribers {
		subs = append(subs, s)
	}
	b.mu.RUnlock()

	delivered := 0
	for _, s := range subs {
		if !s.wants(e.Type) {
			continue
		}
		select {
		case s.Ch <- e:
			delivered++
		default:
			b.mu.Lock()
			b.drops++
			b.mu.Unlock()
		}
	}
	return delivered
}

// SubscriberCount returns the live subscriber count. Used by tests
// and the /api/events/stats debug surface.
func (b *EventBus) SubscriberCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscribers)
}

// Drops returns the cumulative number of events dropped because a
// subscriber's buffer was full. Process-lifetime counter.
func (b *EventBus) Drops() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.drops
}
