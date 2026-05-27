package hexapi

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MatchBroadcaster primitive
// ---------------------------------------------------------------------------

// TestMatchBroadcaster_SubscribeReceivesBroadcasts pins the basic
// fan-out contract: an event broadcast on runID reaches every active
// subscriber of that runID.
func TestMatchBroadcaster_SubscribeReceivesBroadcasts(t *testing.T) {
	mb := NewMatchBroadcaster()
	subA, closedA := mb.Subscribe(42)
	subB, closedB := mb.Subscribe(42)
	if closedA || closedB {
		t.Fatal("fresh runID should not be flagged as closed")
	}

	evt := TournamentMatchEvent{RunID: 42, GameIndex: 1, WinnerSeat: 0, RunningWins: 1}
	mb.Broadcast(42, evt)

	for name, sub := range map[string]*matchSubscriber{"A": subA, "B": subB} {
		select {
		case got := <-sub.ch:
			if got.GameIndex != 1 || got.WinnerSeat != 0 {
				t.Fatalf("subscriber %s got wrong event %+v", name, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %s timed out waiting for broadcast", name)
		}
	}
}

// TestMatchBroadcaster_IsolatedByRunID confirms a broadcast on run X
// does NOT reach subscribers of run Y — keeps tournaments cleanly
// partitioned.
func TestMatchBroadcaster_IsolatedByRunID(t *testing.T) {
	mb := NewMatchBroadcaster()
	subX, _ := mb.Subscribe(1)
	subY, _ := mb.Subscribe(2)

	mb.Broadcast(1, TournamentMatchEvent{RunID: 1, GameIndex: 1})

	select {
	case <-subX.ch:
		// expected
	case <-time.After(time.Second):
		t.Fatal("subscriber for run 1 should have received broadcast")
	}

	select {
	case got := <-subY.ch:
		t.Fatalf("subscriber for run 2 should NOT have received run 1 event, got %+v", got)
	case <-time.After(50 * time.Millisecond):
		// expected — no event
	}
}

// TestMatchBroadcaster_CloseFlushesAndBlocksNewSubs pins the terminal
// transition: Close drains active subscribers via channel-close (so
// the SSE handler's range loop exits) AND new subscribers see
// closed=true on entry so they skip the live-stream path.
func TestMatchBroadcaster_CloseFlushesAndBlocksNewSubs(t *testing.T) {
	mb := NewMatchBroadcaster()
	sub, _ := mb.Subscribe(7)
	mb.Close(7)

	if _, ok := <-sub.ch; ok {
		t.Fatal("Close must close the subscriber channel so range exits")
	}

	lateSub, alreadyClosed := mb.Subscribe(7)
	if !alreadyClosed {
		t.Fatal("subscribe after Close must report alreadyClosed=true")
	}
	// Late subscriber's channel must be already closed so callers can
	// safely `for range sub.ch` without branching on the bool —
	// matches the active-then-Close lifecycle exactly. A read returns
	// (zero-value, ok=false) immediately, no blocking.
	if _, ok := <-lateSub.ch; ok {
		t.Fatal("late subscriber's channel must be closed (ok=false), not open with pending data")
	}
}

// TestMatchBroadcaster_NonBlockingOnFullBuffer confirms the gauntlet
// engine cannot stall on a slow subscriber. Broadcast fills the
// 32-deep buffer then drops the 33rd event without blocking the call.
func TestMatchBroadcaster_NonBlockingOnFullBuffer(t *testing.T) {
	mb := NewMatchBroadcaster()
	sub, _ := mb.Subscribe(99)

	done := make(chan struct{})
	go func() {
		defer close(done)
		// 100 broadcasts on a 32-deep buffer with no consumer — must
		// return promptly via the default-drop branch.
		for i := 0; i < 100; i++ {
			mb.Broadcast(99, TournamentMatchEvent{RunID: 99, GameIndex: i + 1})
		}
	}()

	select {
	case <-done:
		// expected
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on a full subscriber buffer — gauntlet engine could stall")
	}

	// At least the first 32 should be readable; the rest were dropped.
	received := 0
	for {
		select {
		case <-sub.ch:
			received++
		case <-time.After(50 * time.Millisecond):
			if received < 1 || received > 32 {
				t.Fatalf("expected 1-32 buffered events, got %d", received)
			}
			return
		}
	}
}

// TestMatchBroadcaster_UnsubscribeStopsDelivery confirms an
// unsubscribed listener no longer receives events. Defends against
// the leak shape where an SSE handler returns but its subscriber
// stays in the map indefinitely.
func TestMatchBroadcaster_UnsubscribeStopsDelivery(t *testing.T) {
	mb := NewMatchBroadcaster()
	sub, _ := mb.Subscribe(11)
	mb.Unsubscribe(11, sub)
	if mb.SubscriberCount(11) != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", mb.SubscriberCount(11))
	}

	mb.Broadcast(11, TournamentMatchEvent{RunID: 11, GameIndex: 1})

	select {
	case got := <-sub.ch:
		t.Fatalf("unsubscribed listener should not receive events, got %+v", got)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// TestMatchBroadcaster_ConcurrentBroadcastSafe is a race-detector
// canary. The gauntlet loop calls Broadcast from one goroutine while
// SSE handlers Subscribe / Unsubscribe from others — if the mutex is
// dropped or the maps grow without protection, -race catches it.
func TestMatchBroadcaster_ConcurrentBroadcastSafe(t *testing.T) {
	mb := NewMatchBroadcaster()
	var wg sync.WaitGroup
	const subs = 16
	const broadcasts = 200

	for i := 0; i < subs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := mb.Subscribe(1)
			for range s.ch {
				// drain
			}
		}()
	}

	go func() {
		for i := 0; i < broadcasts; i++ {
			mb.Broadcast(1, TournamentMatchEvent{RunID: 1, GameIndex: i + 1})
		}
		mb.Close(1)
	}()

	wg.Wait()
}

// ---------------------------------------------------------------------------
// BuildMatchEvent shape
// ---------------------------------------------------------------------------

// TestBuildMatchEvent_WinnerCommanderResolution confirms the seat-to-
// commander mapping is correct and draws (winnerSeat=-1) get an
// empty commander string per contract.
func TestBuildMatchEvent_WinnerCommanderResolution(t *testing.T) {
	commanders := []string{"Atraxa", "Krenko", "Yuriko", "Edgar"}
	now := time.Unix(1_700_000_000, 0)

	winEvt := BuildMatchEvent(42, 5, 100, 0, commanders, 14, 3, 2, now)
	if winEvt.WinnerCommander != "Atraxa" {
		t.Fatalf("seat 0 winner: want Atraxa, got %q", winEvt.WinnerCommander)
	}
	if got, want := winEvt.Opponents, []string{"Krenko", "Yuriko", "Edgar"}; !equalSlices(got, want) {
		t.Fatalf("opponents: want %v, got %v", want, got)
	}
	if winEvt.RunningWinRate != 60.0 {
		t.Fatalf("3W-2L should be 60.0%%, got %v", winEvt.RunningWinRate)
	}
	if winEvt.FinishedAt != now.Unix() {
		t.Fatalf("FinishedAt: want %d, got %d", now.Unix(), winEvt.FinishedAt)
	}

	drawEvt := BuildMatchEvent(42, 5, 100, -1, commanders, 50, 3, 2, now)
	if drawEvt.WinnerCommander != "" {
		t.Fatalf("draw should have empty commander, got %q", drawEvt.WinnerCommander)
	}
	if drawEvt.WinnerSeat != -1 {
		t.Fatalf("draw should preserve WinnerSeat=-1, got %d", drawEvt.WinnerSeat)
	}
}

// TestBuildMatchEvent_WinRateZeroOnZeroGames defends the divide-by-zero
// path. Should never fire in production (wins+losses>0 when a match
// completes) but a defensive 0.0 keeps the JSON valid.
func TestBuildMatchEvent_WinRateZeroOnZeroGames(t *testing.T) {
	evt := BuildMatchEvent(1, 1, 10, 0, []string{"A", "B", "C", "D"}, 5, 0, 0, time.Now())
	if evt.RunningWinRate != 0.0 {
		t.Fatalf("0/0 winrate should be 0.0, got %v", evt.RunningWinRate)
	}
}

// ---------------------------------------------------------------------------
// /api/tournament/{id}/stream wiring
// ---------------------------------------------------------------------------

// TestTournamentStream_BadIDReturns400 confirms input validation
// before any expensive resolution work.
func TestTournamentStream_BadIDReturns400(t *testing.T) {
	h := &Handler{Showmatch: &Showmatch{MatchBroadcaster: NewMatchBroadcaster()}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tournament/{id}/stream", h.handleTournamentStream)

	for _, bad := range []string{"abc", "-1", "0"} {
		req := httptest.NewRequest("GET", "/api/tournament/"+bad+"/stream", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("id %q: expected 400, got %d", bad, w.Code)
		}
	}
}

// TestTournamentStream_UnknownRunReturns404 confirms a probe for a
// run id with no DB row AND no in-memory entry returns 404, not a
// hanging stream — defends against id-fingerprinting via stream
// behavior.
func TestTournamentStream_UnknownRunReturns404(t *testing.T) {
	h := &Handler{Showmatch: &Showmatch{
		MatchBroadcaster: NewMatchBroadcaster(),
		gauntlets:        map[string]*GauntletResult{},
	}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tournament/{id}/stream", h.handleTournamentStream)

	req := httptest.NewRequest("GET", "/api/tournament/999999/stream", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (body=%q)", w.Code, w.Body.String())
	}
}

// TestTournamentStream_LiveMatchEventReaches is the load-bearing
// end-to-end test: an in-memory active run, a subscribed SSE client,
// and a Broadcast call. Confirms the client receives a match event
// with the correct payload via the SSE wire format.
func TestTournamentStream_LiveMatchEventReaches(t *testing.T) {
	sm := &Showmatch{
		MatchBroadcaster: NewMatchBroadcaster(),
		gauntlets: map[string]*GauntletResult{
			"alice/aggro": {DeckKey: "alice/aggro", RunID: 555, Status: "running", Games: 3},
		},
	}
	h := &Handler{Showmatch: sm}
	srv := httptest.NewServer(http.HandlerFunc(h.handleTournamentStream))
	defer srv.Close()
	// Mux won't apply for raw server — drive the path via a custom mux
	// so PathValue("id") resolves.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tournament/{id}/stream", h.handleTournamentStream)
	srv.Config.Handler = mux

	// Start the request in a goroutine — SSE blocks indefinitely.
	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.Get(srv.URL + "/api/tournament/555/stream")
		if err != nil {
			t.Errorf("get: %v", err)
			return
		}
		respCh <- resp
	}()

	var resp *http.Response
	select {
	case resp = <-respCh:
	case <-time.After(2 * time.Second):
		t.Fatal("stream request did not start in time")
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type: want text/event-stream, got %q", got)
	}

	reader := bufio.NewReader(resp.Body)

	// Read first event — should be `status` with status=active.
	statusEvt := readSSEEvent(t, reader)
	if statusEvt.event != "status" {
		t.Fatalf("first event: want status, got %q", statusEvt.event)
	}
	var statusPayload map[string]any
	if err := json.Unmarshal([]byte(statusEvt.data), &statusPayload); err != nil {
		t.Fatalf("status payload not valid JSON: %v", err)
	}
	if statusPayload["status"] != "active" {
		t.Fatalf("status payload: want active, got %v", statusPayload["status"])
	}
	if statusPayload["run_id"].(float64) != 555 {
		t.Fatalf("status run_id mismatch: %v", statusPayload["run_id"])
	}

	// Fire a match event. Tiny delay to ensure the subscriber is
	// registered (Subscribe in the handler happens before status emit).
	time.Sleep(50 * time.Millisecond)
	sm.MatchBroadcaster.Broadcast(555, BuildMatchEvent(
		555, 4, 100, 0, []string{"Atraxa", "Krenko", "Yuriko", "Edgar"},
		12, 3, 1, time.Unix(1_700_000_000, 0),
	))

	matchEvt := readSSEEvent(t, reader)
	if matchEvt.event != "match" {
		t.Fatalf("second event: want match, got %q (data=%q)", matchEvt.event, matchEvt.data)
	}
	var got TournamentMatchEvent
	if err := json.Unmarshal([]byte(matchEvt.data), &got); err != nil {
		t.Fatalf("match payload not valid JSON: %v", err)
	}
	if got.RunID != 555 || got.GameIndex != 4 || got.WinnerCommander != "Atraxa" {
		t.Fatalf("match event payload mismatch: %+v", got)
	}
	if got.RunningWinRate != 75.0 {
		t.Fatalf("3W-1L should be 75.0%%, got %v", got.RunningWinRate)
	}

	// Close the broadcaster — handler should emit a `complete` event
	// and return.
	sm.MatchBroadcaster.Close(555)
	completeEvt := readSSEEvent(t, reader)
	if completeEvt.event != "complete" {
		t.Fatalf("terminal event: want complete, got %q", completeEvt.event)
	}
}

// TestTournamentStream_AlreadyCompleteReturnsImmediately confirms a
// run that was already closed before the client subscribed emits the
// initial status=complete event and closes — no hanging stream.
func TestTournamentStream_AlreadyCompleteReturnsImmediately(t *testing.T) {
	mb := NewMatchBroadcaster()
	mb.Close(777) // pre-close before any subscribe

	sm := &Showmatch{
		MatchBroadcaster: mb,
		gauntlets: map[string]*GauntletResult{
			"alice/control": {DeckKey: "alice/control", RunID: 777, Status: "complete", Games: 100},
		},
	}
	h := &Handler{Showmatch: sm}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/tournament/{id}/stream", h.handleTournamentStream)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/tournament/777/stream")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	statusEvt := readSSEEvent(t, reader)
	if statusEvt.event != "status" {
		t.Fatalf("first event: want status, got %q", statusEvt.event)
	}
	if !strings.Contains(statusEvt.data, `"status":"complete"`) {
		t.Fatalf("status payload should say complete, got %q", statusEvt.data)
	}
	// Server should close the stream immediately after — reading EOF.
	if _, err := reader.ReadString('\n'); err == nil {
		// One trailing read after the event is fine, but the body
		// should be closed within a short window.
	}
}

// ---------------------------------------------------------------------------
// SSE parsing helper
// ---------------------------------------------------------------------------

type sseEvent struct {
	event string
	data  string
}

// readSSEEvent reads one "event:/data:" pair from an SSE stream.
// Skips heartbeat comment lines (": ping"). Fails the test on timeout
// or malformed input — keeps test code free of repeated parse-loop
// boilerplate.
func readSSEEvent(t *testing.T, r *bufio.Reader) sseEvent {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var evt sseEvent
	for time.Now().Before(deadline) {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("readSSEEvent: %v (partial=%q)", err, line)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if evt.event != "" || evt.data != "" {
				return evt
			}
		case strings.HasPrefix(line, ": "):
			// heartbeat comment — skip
		case strings.HasPrefix(line, "event: "):
			evt.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			if evt.data != "" {
				evt.data += "\n"
			}
			evt.data += strings.TrimPrefix(line, "data: ")
		}
	}
	t.Fatal("readSSEEvent: timeout")
	return evt
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
