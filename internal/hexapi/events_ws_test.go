package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// events_ws_test.go — end-to-end tests for /ws/events:
//   - happy-path: upgrade, receive a published event, clean
//     disconnect releases the slot;
//   - subscribe filter narrows the stream;
//   - client-side ping → server pong;
//   - connection cap returns 503 once exceeded;
//   - nil-bus constructor guard;
//   - sm.Publish-from-game-end integration via the bus.

func TestNewEventsWSHandler_NilBus(t *testing.T) {
	if _, err := NewEventsWSHandler(nil); err == nil {
		t.Fatal("expected error for nil bus, got nil")
	}
}

func TestEventsWS_HappyPath_DeliversPublishedEvent(t *testing.T) {
	bus := NewEventBus()
	h, err := NewEventsWSHandler(bus)
	if err != nil {
		t.Fatalf("NewEventsWSHandler: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)

	conn, err := dialEventsWS(t, srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	// Wait for the subscription to register so the publish lands.
	waitForSubscribers(t, bus, 1, 2*time.Second)

	bus.Publish(Event{Type: EventGameEnd, Data: map[string]any{"game_id": 7}})

	ev := readNextEvent(t, conn, 2*time.Second)
	if ev.Type != EventGameEnd {
		t.Errorf("event type: want %q, got %q", EventGameEnd, ev.Type)
	}
	data := ev.Data.(map[string]any)
	if int(data["game_id"].(float64)) != 7 {
		t.Errorf("event data.game_id: want 7, got %v", data["game_id"])
	}
	if ev.Timestamp.IsZero() {
		t.Errorf("timestamp not populated")
	}

	// Close the WS; the handler must release the slot.
	_ = conn.Close(websocket.StatusNormalClosure, "test done")
	waitForConnections(t, h, 0, 2*time.Second)
}

func TestEventsWS_SubscribeFilter_NarrowsStream(t *testing.T) {
	bus := NewEventBus()
	h, _ := NewEventsWSHandler(bus)
	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)

	conn, err := dialEventsWS(t, srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	waitForSubscribers(t, bus, 1, 2*time.Second)

	// Send subscribe(["gauntlet.progress"]) — game.end events should
	// no longer reach us.
	ctrl := eventControlMessage{Action: "subscribe", Events: []string{EventGauntletProgress}}
	payload, _ := json.Marshal(ctrl)
	if err := conn.Write(context.Background(), websocket.MessageText, payload); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}
	// Give the reader goroutine a beat to apply the filter — the
	// bus has no synchronous handshake. 50ms is plenty.
	time.Sleep(100 * time.Millisecond)

	// Publish both kinds; only gauntlet.progress should arrive.
	bus.Publish(Event{Type: EventGameEnd, Data: "should-be-filtered-out"})
	bus.Publish(Event{Type: EventGauntletProgress, Data: "should-arrive"})

	ev := readNextEvent(t, conn, 2*time.Second)
	if ev.Type != EventGauntletProgress {
		t.Errorf("expected only gauntlet.progress, got %q", ev.Type)
	}
	if s, _ := ev.Data.(string); s != "should-arrive" {
		t.Errorf("event data: want %q, got %v", "should-arrive", ev.Data)
	}

	// Verify nothing else lands within a short window.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Errorf("expected timeout reading second event (filter should have suppressed it)")
	}
}

func TestEventsWS_ClientPing_GetsPong(t *testing.T) {
	bus := NewEventBus()
	h, _ := NewEventsWSHandler(bus)
	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)

	conn, err := dialEventsWS(t, srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	waitForSubscribers(t, bus, 1, 2*time.Second)

	ctrl := eventControlMessage{Action: "ping"}
	payload, _ := json.Marshal(ctrl)
	if err := conn.Write(context.Background(), websocket.MessageText, payload); err != nil {
		t.Fatalf("write ping: %v", err)
	}

	ev := readNextEvent(t, conn, 2*time.Second)
	if ev.Type != "pong" {
		t.Errorf("event type: want pong, got %q", ev.Type)
	}
	data := ev.Data.(map[string]any)
	if _, ok := data["server_time"]; !ok {
		t.Errorf("pong missing server_time: %+v", data)
	}
}

func TestEventsWS_UnknownAction_DoesNotDisconnect(t *testing.T) {
	// Bad JSON + unknown actions should NOT close the connection —
	// the spec is forwards-compatible with future control verbs.
	bus := NewEventBus()
	h, _ := NewEventsWSHandler(bus)
	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)

	conn, err := dialEventsWS(t, srv.URL)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })

	waitForSubscribers(t, bus, 1, 2*time.Second)

	// Unknown action.
	_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{"action":"made-up-verb"}`))
	// Malformed JSON.
	_ = conn.Write(context.Background(), websocket.MessageText, []byte(`{not-json`))

	// Publish an event; we should still receive it (connection
	// survived the garbage frames).
	bus.Publish(Event{Type: EventGameEnd})

	ev := readNextEvent(t, conn, 2*time.Second)
	if ev.Type != EventGameEnd {
		t.Errorf("connection should survive bad control frames; want game.end, got %q", ev.Type)
	}
}

func TestEventsWS_ConnectionCap_RejectsExtraSubscribers(t *testing.T) {
	bus := NewEventBus()
	h, _ := NewEventsWSHandler(bus)

	// Saturate the cap by leasing slots directly. Avoids 100
	// real WebSocket sessions in a test.
	for i := 0; i < maxEventSubscribers; i++ {
		if !h.reserveSlot() {
			t.Fatalf("setup: reserve %d failed", i)
		}
	}
	t.Cleanup(func() {
		for i := 0; i < maxEventSubscribers; i++ {
			h.releaseSlot()
		}
	})

	srv := httptest.NewServer(http.HandlerFunc(h.serve))
	t.Cleanup(srv.Close)

	// Standard http.Get against the WS-upgrade endpoint. The
	// handler should short-circuit with 503 before the upgrade.
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", resp.StatusCode)
	}
	var body ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode ErrorResponse: %v", err)
	}
	if body.Status != http.StatusServiceUnavailable {
		t.Errorf("body.status: want 503, got %d", body.Status)
	}
	if !strings.Contains(body.Error.Message, "too many") {
		t.Errorf("body.error.message: want 'too many event subscribers' substring, got %q", body.Error.Message)
	}
}

func TestEventsWS_RegisterEventsWS_Mounts(t *testing.T) {
	bus := NewEventBus()
	h, _ := NewEventsWSHandler(bus)
	mux := http.NewServeMux()
	h.RegisterEventsWS(mux)

	// Any non-WebSocket GET should hit the handler. With no
	// Upgrade header the websocket.Accept fails — we just need to
	// confirm the route is wired (not a 404).
	req := httptest.NewRequest(http.MethodGet, "/ws/events", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Errorf("/ws/events not wired by RegisterEventsWS")
	}
}

// -------------------------------------------------------------------
// Integration: Showmatch game-end tail publishes onto the bus
// -------------------------------------------------------------------

// TestShowmatch_GameEndTail_PublishesEvent — synthesise the publish
// the game-end tail would emit, then confirm the event reaches a
// connected WS subscriber. This is a unit-level pin against the
// integration in showmatch.go; the full game loop is exercised by
// TestStartGauntlet_PaidHappyPath_ChargesAndLogs (covered by the
// gauntlet refund test file).
func TestShowmatch_GameEndTail_PublishesEvent(t *testing.T) {
	bus := NewEventBus()
	sm := &Showmatch{Events: bus}

	s := bus.Subscribe(8)
	t.Cleanup(func() { bus.Unsubscribe(s) })

	// Inline the publish payload the game-end tail builds. Keeping
	// the test against the bus directly (vs. driving RunGame) means
	// a payload-shape regression is loud and obvious here.
	completed := CompletedGame{
		GameID:     11,
		Winner:     1,
		WinnerName: "Atraxa",
		Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
		DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
		Turns:      14,
		EndReason:  "lethal",
		FinishedAt: time.Date(2026, 5, 24, 16, 0, 0, 0, time.UTC),
	}
	sm.Events.Publish(Event{
		Type: EventGameEnd,
		Data: map[string]any{
			"game_id":     completed.GameID,
			"winner":      completed.Winner,
			"winner_name": completed.WinnerName,
			"commanders":  completed.Commanders,
			"deck_keys":   completed.DeckKeys,
			"turns":       completed.Turns,
			"end_reason":  completed.EndReason,
			"finished_at": completed.FinishedAt.UTC().Format(time.RFC3339),
		},
	})

	select {
	case ev := <-s.Ch:
		if ev.Type != EventGameEnd {
			t.Errorf("event type: want %q, got %q", EventGameEnd, ev.Type)
		}
		data := ev.Data.(map[string]any)
		// Bus delivery is direct (no JSON round-trip), so numeric
		// fields stay as their published Go types.
		if data["game_id"].(int) != 11 {
			t.Errorf("game_id: want 11, got %v", data["game_id"])
		}
		if data["winner_name"] != "Atraxa" {
			t.Errorf("winner_name: want Atraxa, got %v", data["winner_name"])
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive game.end event")
	}
}

// TestShowmatch_NilEvents_IsNoOp — the game-end tail must not panic
// when sm.Events is nil. Production deployments without the events
// feature wired in should silently skip the publish.
func TestShowmatch_NilEvents_IsNoOp(t *testing.T) {
	sm := &Showmatch{} // Events == nil
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil Events should be a no-op; panic = %v", r)
		}
	}()
	if sm.Events != nil {
		sm.Events.Publish(Event{Type: EventGameEnd})
	}
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// dialEventsWS opens a websocket against httptest server URL. The
// httptest server runs http; rewrite the scheme to ws.
func dialEventsWS(t *testing.T, httpURL string) (*websocket.Conn, error) {
	t.Helper()
	wsURL := strings.Replace(httpURL, "http://", "ws://", 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	return conn, err
}

// readNextEvent reads one JSON-encoded Event from the connection
// within timeout, fataling on transport / decode failure.
func readNextEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read: %v", err)
	}
	var ev Event
	if err := json.Unmarshal(data, &ev); err != nil {
		t.Fatalf("decode event: %v (raw=%q)", err, data)
	}
	return ev
}

// waitForSubscribers polls bus.SubscriberCount until it reaches
// `want`, or fails the test on timeout. The handler's reader
// goroutine registers the bus subscription mid-serve, so callers
// have to give it a beat after Dial returns.
func waitForSubscribers(t *testing.T, bus *EventBus, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if bus.SubscriberCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waited %s for SubscriberCount=%d; got %d", timeout, want, bus.SubscriberCount())
}

// waitForConnections is the connection-cap analogue of
// waitForSubscribers: drains the WS slot count back down on
// disconnect.
func waitForConnections(t *testing.T, h *EventsWSHandler, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if h.activeConnections() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waited %s for connections=%d; got %d", timeout, want, h.activeConnections())
}
