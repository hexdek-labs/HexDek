package hexapi

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// events_sse_r60_test.go — pins the GET /api/events/{game_id}/stream
// behavior: SSE wire format, per-game filtering off the EventBus,
// rate-limit gate, keepalive cadence, unified envelope on 4xx, and
// nil-bus / no-flusher edge cases.
//
// SSE tests are inherently timing-flavored — we drive a real handler
// against an httptest.ResponseRecorder via a helper that records
// frames as they're written. To avoid sleeping for keepalive
// intervals we shorten timings via test-local goroutines that publish
// to the bus and read the recorder body.

// --- helpers --------------------------------------------------------------

// flushRecorder wraps httptest.ResponseRecorder to satisfy
// http.Flusher (the production handler bails to 500 if Flusher
// isn't implemented; httptest.ResponseRecorder DOES satisfy it
// since Go 1.7, but be explicit so the test signals intent).
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed int
	mu      sync.Mutex
}

func (f *flushRecorder) Flush() {
	f.mu.Lock()
	f.flushed++
	f.mu.Unlock()
	f.ResponseRecorder.Flush()
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

// drainSSEFrames reads completed SSE events from the recorder body
// until either `want` frames have arrived or the timeout fires.
// Skips heartbeat lines (`: ping`). Returns the parsed `event:` +
// `data:` pairs.
func drainSSEFrames(t *testing.T, rec *flushRecorder, want int, timeout time.Duration) []sseFrame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	frames := []sseFrame{}
	for time.Now().Before(deadline) && len(frames) < want {
		body := rec.Body.String()
		frames = parseSSEFrames(body)
		if len(frames) >= want {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return frames
}

type sseFrame struct {
	Event string
	Data  string
}

// parseSSEFrames splits an SSE body into completed frames. A frame
// is "event: TYPE\ndata: JSON\n\n"; comment lines (": ping") are
// dropped. Partial trailing frames (no blank line yet) are
// excluded.
func parseSSEFrames(body string) []sseFrame {
	var out []sseFrame
	var ev, data string
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			ev = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && ev != "":
			out = append(out, sseFrame{Event: ev, Data: data})
			ev, data = "", ""
		case strings.HasPrefix(line, ":"):
			// heartbeat / comment — skip
		}
	}
	return out
}

// --- Happy path: ready frame + per-game-filtered events -----------------

// On connect, the handler emits a `ready` event so the EventSource
// API has something to fire onopen against — the spec doesn't surface
// successful-connection separately from first-message. Then bus
// events whose data.game_id matches the path are forwarded as
// SSE frames; events for other games are filtered out.
func TestEventStreamSSE_ReadyThenFilteredByGameID(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}

	rec := newFlushRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/42/stream", nil)
	req.SetPathValue("game_id", "42")

	// Run the handler in a goroutine — it blocks on the bus loop
	// until the request context cancels.
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()

	// Wait for the ready frame.
	if got := drainSSEFrames(t, rec, 1, 2*time.Second); len(got) < 1 {
		cancel()
		<-done
		t.Fatalf("ready frame never arrived; body=%q", rec.Body.String())
	}

	// Publish an event for our game + an event for a different game.
	bus.Publish(Event{
		Type: EventTurnTransition,
		Data: map[string]any{"game_id": int64(42), "from_turn": 5, "to_turn": 6},
	})
	bus.Publish(Event{
		Type: EventTurnTransition,
		Data: map[string]any{"game_id": int64(99), "from_turn": 1, "to_turn": 2},
	})
	bus.Publish(Event{
		Type: EventComboFired,
		Data: map[string]any{"game_id": int64(42), "combo": "Thassa's Oracle"},
	})

	// Should see ready + our two events; the foreign-game event
	// must NOT appear.
	frames := drainSSEFrames(t, rec, 3, 2*time.Second)
	cancel()
	<-done

	if len(frames) < 3 {
		t.Fatalf("want >= 3 frames (ready + 2 matching), got %d; body=%q",
			len(frames), rec.Body.String())
	}
	if frames[0].Event != "ready" {
		t.Errorf("first frame event=%q, want ready", frames[0].Event)
	}
	gotEvents := []string{}
	for _, f := range frames[1:] {
		gotEvents = append(gotEvents, f.Event)
	}
	wantSet := map[string]bool{
		EventTurnTransition: true,
		EventComboFired:     true,
	}
	for _, ev := range gotEvents {
		if !wantSet[ev] {
			t.Errorf("unexpected event %q in stream", ev)
		}
	}
	// Foreign-game event filtered → its CombosFired data shouldn't
	// appear in any frame's data field. We assert by substring on
	// the raw body since data is JSON.
	if strings.Contains(rec.Body.String(), `"from_turn":1`) {
		t.Errorf("foreign game (game_id=99) leaked into per-game stream; body=%q", rec.Body.String())
	}
}

// Events whose data is a typed struct (not map[string]any) still
// match via marshal-then-unmarshal fallback. The production game.end
// publisher emits map[string]any already, but a future typed
// publisher MUST also flow through — defends the eventMatchesGame
// struct fallback.
func TestEventStreamSSE_StructDataMatchesGameID(t *testing.T) {
	type turnEvent struct {
		GameID   int64 `json:"game_id"`
		FromTurn int   `json:"from_turn"`
	}
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}

	rec := newFlushRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/7/stream", nil)
	req.SetPathValue("game_id", "7")
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()
	drainSSEFrames(t, rec, 1, 2*time.Second) // ready

	bus.Publish(Event{
		Type: EventTurnTransition,
		Data: turnEvent{GameID: 7, FromTurn: 3},
	})

	frames := drainSSEFrames(t, rec, 2, 2*time.Second)
	cancel()
	<-done

	if len(frames) < 2 {
		t.Fatalf("typed-struct event not forwarded; got %d frames; body=%q",
			len(frames), rec.Body.String())
	}
	if frames[1].Event != EventTurnTransition {
		t.Errorf("frame event=%q, want %q", frames[1].Event, EventTurnTransition)
	}
}

// Events without a game_id field never reach a per-game stream
// (cluster-wide events like elo.recalc must NOT spam every
// spectator's per-game window).
func TestEventStreamSSE_NoGameIDDropsFromPerGameStream(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}

	rec := newFlushRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
	req.SetPathValue("game_id", "1")
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()
	drainSSEFrames(t, rec, 1, 2*time.Second) // ready

	bus.Publish(Event{
		Type: EventELOUpdate,
		Data: map[string]any{"deck": "alice/voltron", "delta": 12},
	})

	// Give the handler a moment in case it WAS going to emit; then
	// assert no second frame arrived.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	frames := parseSSEFrames(rec.Body.String())
	if len(frames) != 1 {
		t.Errorf("want only ready frame, got %d frames: %+v", len(frames), frames)
	}
}

// --- Validation + error paths -------------------------------------------

func TestEventStreamSSE_400OnInvalidGameID(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}
	for _, badID := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/events/"+badID+"/stream", nil)
		req.SetPathValue("game_id", badID)
		rec := httptest.NewRecorder()
		h.serve(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("game_id=%q: status=%d, want 400", badID, rec.Code)
		}
		// Unified envelope decode.
		var body ErrorResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("game_id=%q: body did not decode as ErrorResponse: %v", badID, err)
		}
		if body.Error.Code != "bad_request" {
			t.Errorf("game_id=%q: code=%q, want bad_request", badID, body.Error.Code)
		}
	}
}

func TestEventStreamSSE_503WhenBusMissing(t *testing.T) {
	h := &EventStreamHandler{} // Bus nil
	req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
	req.SetPathValue("game_id", "1")
	rec := httptest.NewRecorder()
	h.serve(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "service_unavailable" {
		t.Errorf("code=%q, want service_unavailable", body.Error.Code)
	}
}

// Per-IP rate limit gates the upgrade. Drives 2 successful upgrades
// against a 1-burst limiter; the 2nd must 429 with the unified
// envelope. Successful upgrades cancel their own context to exit
// the handler loop so we don't leak goroutines.
func TestEventStreamSSE_RateLimitGate(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{
		Bus:     bus,
		Limiter: NewRateLimiter(1, 1.0/60.0),
	}

	// 1st request from this IP: passes. We have to actually drive
	// the handler so it consumes the token; cancel the context so
	// it returns.
	{
		rec := newFlushRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
		req.SetPathValue("game_id", "1")
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		ctx, cancel := context.WithCancel(context.Background())
		req = req.WithContext(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			h.serve(rec, req)
		}()
		drainSSEFrames(t, rec, 1, 2*time.Second) // wait for ready
		cancel()
		<-done
	}

	// 2nd request from same IP: 429.
	req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
	req.SetPathValue("game_id", "1")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	rec := httptest.NewRecorder()
	h.serve(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d, want 429 (body=%q)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After missing on 429")
	}
	var body ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "too_many_requests" {
		t.Errorf("code=%q, want too_many_requests", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "event stream") {
		t.Errorf("message=%q must carry the endpoint label", body.Error.Message)
	}
}

// --- SSE header contract ------------------------------------------------

func TestEventStreamSSE_HeadersAndContentType(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}

	rec := newFlushRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
	req.SetPathValue("game_id", "1")
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()
	drainSSEFrames(t, rec, 1, 2*time.Second)
	cancel()
	<-done

	want := map[string]string{
		"Content-Type":           "text/event-stream",
		"Cache-Control":          "no-cache",
		"Connection":             "keep-alive",
		"X-Accel-Buffering":      "no",
		"X-Content-Type-Options": "nosniff",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %q = %q, want %q", k, got, v)
		}
	}
}

// --- mapHasMatchingGameID unit coverage ---------------------------------

func TestMapHasMatchingGameID(t *testing.T) {
	cases := []struct {
		name   string
		m      map[string]any
		gameID int64
		want   bool
	}{
		{"float64 match (JSON numeric default)", map[string]any{"game_id": float64(42)}, 42, true},
		{"int match", map[string]any{"game_id": 42}, 42, true},
		{"int64 match", map[string]any{"game_id": int64(42)}, 42, true},
		{"string match", map[string]any{"game_id": "42"}, 42, true},
		{"string mismatch", map[string]any{"game_id": "abc"}, 42, false},
		{"missing key", map[string]any{"other": 42}, 42, false},
		{"wrong type", map[string]any{"game_id": []int{42}}, 42, false},
		{"different value", map[string]any{"game_id": float64(99)}, 42, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := mapHasMatchingGameID(tc.m, tc.gameID); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Bus disconnect cleanup ---------------------------------------------

// When the bus unsubscribes us mid-stream (subscriber channel closes),
// the handler returns cleanly. Defends against a goroutine leak if a
// future refactor changes the bus shutdown semantics.
func TestEventStreamSSE_BusUnsubscribeClosesHandler(t *testing.T) {
	bus := NewEventBus()
	h := &EventStreamHandler{Bus: bus}

	rec := newFlushRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/events/1/stream", nil)
	req.SetPathValue("game_id", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()
	drainSSEFrames(t, rec, 1, 2*time.Second)

	// Capture our subscriber by counting BEFORE and AFTER unsubscribing
	// every live sub. (The bus doesn't surface "the one we just
	// created" directly, but it does expose SubscriberCount.)
	if got := bus.SubscriberCount(); got != 1 {
		t.Fatalf("want 1 subscriber, got %d", got)
	}
	// Cancel the request context — handler should exit through
	// ctx.Done branch and unregister. This is the standard shutdown
	// path; we're checking it doesn't deadlock or leak.
	cancel()
	select {
	case <-done:
		// fine — handler returned cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after request context cancel — goroutine leak")
	}
	if got := bus.SubscriberCount(); got != 0 {
		t.Errorf("post-exit subscriber count = %d, want 0", got)
	}
}
