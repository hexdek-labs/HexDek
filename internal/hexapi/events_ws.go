package hexapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// events_ws.go — /ws/events WebSocket bridge over the EventBus.
//
// Wire protocol:
//
//   Server → client: one JSON-encoded Event per message
//       {"event":"game.end","data":{...},"timestamp":"2026-05-24T15:00:00Z"}
//
//   Client → server: optional control messages
//       {"action":"subscribe", "events":["game.end","gauntlet.progress"]}
//       {"action":"subscribe", "events":[]}    // subscribe-to-all
//       {"action":"ping"}                       // server replies {"event":"pong",...}
//
//   The default subscription on connect is "all events" — clients
//   that only care about a subset can narrow with subscribe, but
//   the dashboard happy-path doesn't have to send anything.

const (
	// maxEventSubscribers caps live /ws/events connections. The
	// existing /ws/live surface uses 100; this stream serves a
	// narrower use case (dashboards + bots), so we set the same
	// budget rather than introduce a second tunable.
	maxEventSubscribers = 100

	// eventWSKeepalive controls how often the server sends a
	// keepalive ping. Match nginx's default 60s idle close.
	eventWSKeepalive = 30 * time.Second

	// eventWSWriteTimeout is the per-message write budget. Slow
	// consumers either drain or get evicted via context timeout.
	eventWSWriteTimeout = 5 * time.Second

	// eventWSReadLimit caps incoming control-message size. 4 KiB
	// is plenty for a subscribe with even a few hundred event-type
	// strings.
	eventWSReadLimit = 4 * 1024
)

// EventsWSHandler is the http.Handler returned by RegisterEventsWS.
// Wraps an EventBus plus a counter to enforce the connection cap.
type EventsWSHandler struct {
	bus *EventBus

	// connMu guards connections — bumped in serve when a slot is
	// reserved, decremented when the session exits.
	connMu      sync.Mutex
	connections int
}

// NewEventsWSHandler returns the handler. Bus must be non-nil; a nil
// bus is a wiring error the caller should fail-fast on rather than
// silently route /ws/events into a 500.
func NewEventsWSHandler(bus *EventBus) (*EventsWSHandler, error) {
	if bus == nil {
		return nil, errors.New("events ws: nil EventBus")
	}
	return &EventsWSHandler{bus: bus}, nil
}

// RegisterEventsWS mounts GET /ws/events on mux.
func (h *EventsWSHandler) RegisterEventsWS(mux *http.ServeMux) {
	mux.HandleFunc("GET /ws/events", h.serve)
}

// activeConnections is the read-side accessor for the live count.
func (h *EventsWSHandler) activeConnections() int {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	return h.connections
}

func (h *EventsWSHandler) reserveSlot() bool {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.connections >= maxEventSubscribers {
		return false
	}
	h.connections++
	return true
}

func (h *EventsWSHandler) releaseSlot() {
	h.connMu.Lock()
	defer h.connMu.Unlock()
	if h.connections > 0 {
		h.connections--
	}
}

// serve handles one WebSocket session: upgrade, subscribe to the
// bus, fan messages between the bus channel and the client until
// either side closes.
func (h *EventsWSHandler) serve(w http.ResponseWriter, r *http.Request) {
	if !h.reserveSlot() {
		writeError(w, http.StatusServiceUnavailable, "too many event subscribers")
		return
	}
	defer h.releaseSlot()

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("events ws: upgrade error: %v", err)
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(eventWSReadLimit)

	sub := h.bus.Subscribe(64)
	defer h.bus.Unsubscribe(sub)

	// Reader goroutine: handles client control messages (subscribe,
	// ping). When the reader returns, we cancel the writer's
	// context so the writer loop drains and exits.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		readEventControlMessages(ctx, conn, sub)
	}()

	keepalive := time.NewTicker(eventWSKeepalive)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-readerDone:
			// Client closed the connection or sent a malformed
			// frame; drop the session.
			return
		case ev, ok := <-sub.Ch:
			if !ok {
				return
			}
			if err := writeEventMessage(ctx, conn, ev); err != nil {
				return
			}
		case <-keepalive.C:
			pingEvent := Event{
				Type:      "ping",
				Timestamp: time.Now().UTC(),
				Data:      map[string]int64{"server_time": time.Now().Unix()},
			}
			if err := writeEventMessage(ctx, conn, pingEvent); err != nil {
				return
			}
		}
	}
}

// eventControlMessage is the JSON shape clients send for subscribe
// / ping. Unknown actions are ignored — forwards-compat with
// future control verbs.
type eventControlMessage struct {
	Action string   `json:"action"`
	Events []string `json:"events,omitempty"`
}

func readEventControlMessages(ctx context.Context, conn *websocket.Conn, sub *EventSubscriber) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var msg eventControlMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			// Bad JSON — ignore the frame, keep the connection
			// alive. The client can retry with a valid message.
			continue
		}
		switch msg.Action {
		case "subscribe":
			sub.SetFilter(msg.Events)
		case "ping":
			// Pong inline via the bus is overkill; just enqueue a
			// pong event into the subscriber's own channel so the
			// writer goroutine handles it on the next select.
			select {
			case sub.Ch <- Event{
				Type:      "pong",
				Timestamp: time.Now().UTC(),
				Data:      map[string]int64{"server_time": time.Now().Unix()},
			}:
			default:
				// Buffer full — drop the pong. The keepalive
				// ticker will exercise liveness independently.
			}
		default:
			// Unknown action; ignore.
		}
	}
}

// writeEventMessage marshals + writes one Event with a bounded
// timeout. Returns an error on transport failure so the caller can
// drop the session.
func writeEventMessage(ctx context.Context, conn *websocket.Conn, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, eventWSWriteTimeout)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, body)
}
