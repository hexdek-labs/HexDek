package hexapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// r60 round 2 — table-driven coverage for the three tournament-shaped
// hexapi endpoints exposed by Showmatch.RegisterShowmatch:
//
//   POST /api/gauntlet/{owner}/{id}            — handleStartGauntlet
//   GET  /api/gauntlet/{owner}/{id}            — handleGetGauntlet
//   GET  /api/tournaments/{owner}/{id}/events  — handleTournamentEvents (SSE)
//
// Existing tests cover handleStartGauntlet's path validation
// (showmatch_gauntlet_validation_r60_test.go), the credit-charge
// refund cases (showmatch_gauntlet_refund_r60_test.go), and the 402
// envelope shape (showmatch_gauntlet_credits_envelope_r60_test.go).
// This file fills the remaining gaps:
//
//   - handleGetGauntlet happy path with a registered gauntlet result
//     (the existing test only exercises the "none" branch).
//   - handleStartGauntlet free-tier happy path (CanRunFree=true, no
//     credit debit, response omits credits_charged).
//   - handleStartGauntlet running-already short-circuit (returns 200
//     with the in-flight result, no second goroutine launched).
//   - handleTournamentEvents path validation — pre-fix this handler
//     did NOT run validatePathComponent so an owner like ".." or an
//     id with a slash could pollute the SSE topic map. Mirrors the
//     guard the two sibling gauntlet handlers already run.
//   - handleTournamentEvents streaming-unsupported 500 path (via a
//     ResponseWriter that intentionally does not implement Flusher).
//   - handleTournamentEvents SSE happy paths: an immediate
//     "snapshot" frame for the no-gauntlet case, an immediate
//     terminal snapshot when the gauntlet is already complete, and
//     a live broadcast that closes the stream when the deck reaches
//     a terminal status.

// -------------------------------------------------------------------
// handleGetGauntlet — happy path with a registered result
// -------------------------------------------------------------------

func TestHandleGetGauntlet_RegisteredResult(t *testing.T) {
	sm := &Showmatch{
		gauntlets: map[string]*GauntletResult{},
	}

	finished := time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC)
	started := finished.Add(-30 * time.Minute)
	sm.gauntlets["alice/korvold"] = &GauntletResult{
		DeckKey:    "alice/korvold",
		Commander:  "Korvold, Fae-Cursed King",
		Status:     "complete",
		Games:      100,
		Target:     100,
		Wins:       42,
		Losses:     58,
		WinRate:    0.42,
		ELOStart:   1500,
		ELOEnd:    1522,
		ELODelta:  22,
		AvgTurns:  12.4,
		StartedAt: started,
		FinishedAt: finished,
		Placements: [4]int{42, 30, 18, 10},
	}

	cases := []struct {
		name       string
		owner      string
		id         string
		wantStatus int
		assertResp func(t *testing.T, body map[string]any)
	}{
		{
			name:       "happy path — registered gauntlet returned verbatim",
			owner:      "alice",
			id:         "korvold",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, body map[string]any) {
				if got, _ := body["status"].(string); got != "complete" {
					t.Errorf("status: want complete, got %q", got)
				}
				if got, _ := body["deck_key"].(string); got != "alice/korvold" {
					t.Errorf("deck_key: want alice/korvold, got %q", got)
				}
				if got, _ := body["wins"].(float64); int(got) != 42 {
					t.Errorf("wins: want 42, got %v", body["wins"])
				}
				if got, _ := body["losses"].(float64); int(got) != 58 {
					t.Errorf("losses: want 58, got %v", body["losses"])
				}
				if got, _ := body["elo_delta"].(float64); got != 22 {
					t.Errorf("elo_delta: want 22, got %v", body["elo_delta"])
				}
			},
		},
		{
			name:       "no gauntlet registered — status=none",
			owner:      "alice",
			id:         "bareheaded_pioneer",
			wantStatus: http.StatusOK,
			assertResp: func(t *testing.T, body map[string]any) {
				if got, _ := body["status"].(string); got != "none" {
					t.Errorf("status: want none, got %q", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/gauntlet/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id), nil)
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			sm.handleGetGauntlet(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			var body map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
			}
			tc.assertResp(t, body)
		})
	}
}

// -------------------------------------------------------------------
// handleStartGauntlet — free-tier and already-running happy paths
// -------------------------------------------------------------------

func TestHandleStartGauntlet_FreeTier_NoCreditCharge(t *testing.T) {
	f := newGauntletRefundFixture(t)
	// Fresh fixture has 0 free-tier rows for today, so CanRunFree=true
	// without any seeding. Topping the balance is unnecessary because
	// the handler never reaches credits.Spend.
	f.addDeckToPool("alice", "korvold", "Korvold")

	baselineBalance := f.currentBalance(t)
	baselineUsage := f.gauntletUsageRowsFor(t)

	req := f.makeStartReq("alice", "korvold")
	rr := httptest.NewRecorder()
	f.sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
	}
	if got, _ := resp["status"].(string); got != "started" {
		t.Errorf("status: want started, got %q", got)
	}
	if _, charged := resp["credits_charged"]; charged {
		t.Errorf("free-tier response leaked credits_charged: %+v", resp)
	}
	if got, _ := resp["deck_key"].(string); got != "alice/korvold" {
		t.Errorf("deck_key: want alice/korvold, got %q", got)
	}

	if got := f.currentBalance(t); got != baselineBalance {
		t.Errorf("free-tier debited balance: baseline=%d, after=%d", baselineBalance, got)
	}
	if got := f.gauntletUsageRowsFor(t); got != baselineUsage+1 {
		t.Errorf("free-tier should still log a usage row: baseline=%d, after=%d",
			baselineUsage, got)
	}

	waitForGauntletStatus(t, f.sm, "alice/korvold", "complete", 2*time.Second)
}

func TestHandleStartGauntlet_AlreadyRunning_ReturnsExisting(t *testing.T) {
	sm := &Showmatch{
		gauntlets: map[string]*GauntletResult{},
	}
	// Seed an in-flight gauntlet. The handler short-circuits before any
	// state mutation, so a bare Showmatch (no credits, no deckPool, no
	// gauntletSem owned) is sufficient.
	sm.gauntlets["alice/korvold"] = &GauntletResult{
		DeckKey: "alice/korvold",
		Status:  "running",
		Games:   42,
		Target:  500,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/gauntlet/alice/korvold", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")
	rr := httptest.NewRecorder()

	sm.handleStartGauntlet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200 (existing gauntlet returned), got %d", rr.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
	}
	if got, _ := body["status"].(string); got != "running" {
		t.Errorf("status: want running, got %q", got)
	}
	if got, _ := body["games"].(float64); int(got) != 42 {
		t.Errorf("games (progress) should be 42, got %v", body["games"])
	}
	// Critical: no goroutine should have launched (would re-RUN the
	// gauntlet), and no result should have been overwritten.
	sm.gauntletMu.RLock()
	res := sm.gauntlets["alice/korvold"]
	sm.gauntletMu.RUnlock()
	if res == nil || res.Status != "running" || res.Games != 42 {
		t.Errorf("existing gauntlet was clobbered: %+v", res)
	}
}

// -------------------------------------------------------------------
// handleTournamentEvents — path validation
// -------------------------------------------------------------------

func TestHandleTournamentEvents_PathValidation(t *testing.T) {
	sm := &Showmatch{}

	cases := []struct {
		name       string
		owner      string
		id         string
		wantStatus int
		wantErrMsg string
	}{
		{
			name:       "owner with .. rejected",
			owner:      "..",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "id with slash rejected (SSE topic pollution)",
			owner:      "alice",
			id:         "korvold/extra",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "empty owner rejected",
			owner:      "",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "empty id rejected",
			owner:      "alice",
			id:         "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "owner with backslash rejected",
			owner:      "alice\\evil",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "id with NUL byte rejected",
			owner:      "alice",
			id:         "korvold\x00",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/tournaments/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id)+"/events", nil)
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			sm.handleTournamentEvents(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			var body ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
			}
			if body.Error != tc.wantErrMsg {
				t.Errorf("body.error: want %q, got %q", tc.wantErrMsg, body.Error)
			}
			if body.Status != tc.wantStatus {
				t.Errorf("body.status: want %d, got %d", tc.wantStatus, body.Status)
			}
			// No subscription should have been registered.
			sm.gauntletSubsMu.Lock()
			n := len(sm.gauntletSubs)
			sm.gauntletSubsMu.Unlock()
			if n != 0 {
				t.Errorf("subscriber map gained an entry on a rejected request: %d", n)
			}
		})
	}
}

// -------------------------------------------------------------------
// handleTournamentEvents — streaming-unsupported 500
// -------------------------------------------------------------------

// nonFlusherRecorder wraps an httptest.ResponseRecorder but
// intentionally hides the Flush method. http.Flusher is an
// interface assertion against the writer, and httptest.NewRecorder
// satisfies it by default; we shadow it to exercise the "streaming
// unsupported" branch.
type nonFlusherRecorder struct {
	rr *httptest.ResponseRecorder
}

func (n *nonFlusherRecorder) Header() http.Header         { return n.rr.Header() }
func (n *nonFlusherRecorder) Write(b []byte) (int, error) { return n.rr.Write(b) }
func (n *nonFlusherRecorder) WriteHeader(code int)        { n.rr.WriteHeader(code) }

func TestHandleTournamentEvents_StreamingUnsupported(t *testing.T) {
	sm := &Showmatch{}
	req := httptest.NewRequest(http.MethodGet, "/api/tournaments/alice/korvold/events", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")

	rr := httptest.NewRecorder()
	w := &nonFlusherRecorder{rr: rr}

	// Sanity: confirm w does NOT satisfy http.Flusher so the test
	// actually drives the intended branch.
	if _, ok := http.ResponseWriter(w).(http.Flusher); ok {
		t.Fatalf("nonFlusherRecorder must not implement http.Flusher")
	}

	sm.handleTournamentEvents(w, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
	}
	if body.Error != "streaming unsupported" {
		t.Errorf("body.error: want %q, got %q", "streaming unsupported", body.Error)
	}
	if body.Status != http.StatusInternalServerError {
		t.Errorf("body.status: want 500, got %d", body.Status)
	}
}

// -------------------------------------------------------------------
// handleTournamentEvents — SSE happy paths
// -------------------------------------------------------------------

// sseRecorder is a Flusher-satisfying ResponseWriter with thread-safe
// buffered writes. The handler runs in a goroutine and the test reads
// frames from the buffer; the mutex keeps the io.Writer race-free.
type sseRecorder struct {
	hdr  http.Header
	code int
	mu   sync.Mutex
	buf  bytes.Buffer
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{hdr: http.Header{}, code: http.StatusOK}
}

func (s *sseRecorder) Header() http.Header { return s.hdr }
func (s *sseRecorder) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(b)
}
func (s *sseRecorder) WriteHeader(code int) { s.code = code }
func (s *sseRecorder) Flush()               {}
func (s *sseRecorder) body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// awaitSSEFrame polls the recorder until the body contains a complete
// `event: <name>\ndata: ...\n\n` frame for the given event, or the
// timeout elapses.
func awaitSSEFrame(t *testing.T, w *sseRecorder, event string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	prefix := "event: " + event + "\n"
	for time.Now().Before(deadline) {
		body := w.body()
		if idx := strings.Index(body, prefix); idx >= 0 {
			// Find the terminating blank line for this frame.
			rest := body[idx:]
			if end := strings.Index(rest, "\n\n"); end >= 0 {
				return rest[:end+2]
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("SSE frame %q not observed within %s; body=%q", event, timeout, w.body())
	return ""
}

func TestHandleTournamentEvents_NoGauntlet_EmitsNoneSnapshot(t *testing.T) {
	sm := &Showmatch{
		gauntlets:    map[string]*GauntletResult{},
		gauntletSubs: map[string]map[*gauntletSubscriber]struct{}{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet,
		"/api/tournaments/alice/korvold/events", nil).WithContext(ctx)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")

	w := newSSERecorder()
	done := make(chan struct{})
	go func() {
		sm.handleTournamentEvents(w, req)
		close(done)
	}()

	// First frame must be the snapshot with status=none.
	frame := awaitSSEFrame(t, w, "snapshot", 2*time.Second)
	if !strings.Contains(frame, `"status":"none"`) {
		t.Errorf("first snapshot frame missing status=none: %q", frame)
	}
	if !strings.Contains(frame, `"deck_key":"alice/korvold"`) {
		t.Errorf("first snapshot frame missing deck_key: %q", frame)
	}
	// SSE headers must be set.
	if got := w.hdr.Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type: want text/event-stream, got %q", got)
	}
	if got := w.hdr.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control: want no-cache, got %q", got)
	}
	if got := w.hdr.Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering: want no, got %q", got)
	}

	// Subscription must have been registered.
	sm.gauntletSubsMu.Lock()
	hasSub := len(sm.gauntletSubs["alice/korvold"]) > 0
	sm.gauntletSubsMu.Unlock()
	if !hasSub {
		t.Errorf("expected one SSE subscriber registered for alice/korvold")
	}

	// Cancel the request context — handler must drain and return so
	// the goroutine doesn't leak past the test.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("handler did not exit after context cancel")
	}

	// Subscription must be cleaned up.
	sm.gauntletSubsMu.Lock()
	stillSubbed := len(sm.gauntletSubs["alice/korvold"])
	sm.gauntletSubsMu.Unlock()
	if stillSubbed != 0 {
		t.Errorf("subscription leaked after handler exit: %d still registered", stillSubbed)
	}
}

func TestHandleTournamentEvents_TerminalGauntlet_ExitsImmediately(t *testing.T) {
	sm := &Showmatch{
		gauntlets: map[string]*GauntletResult{
			"alice/korvold": {
				DeckKey:   "alice/korvold",
				Commander: "Korvold",
				Status:    "complete",
				Games:     100,
				Wins:      55,
				Losses:    45,
			},
		},
		gauntletSubs: map[string]map[*gauntletSubscriber]struct{}{},
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/tournaments/alice/korvold/events", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")

	w := newSSERecorder()
	done := make(chan struct{})
	go func() {
		sm.handleTournamentEvents(w, req)
		close(done)
	}()

	// Handler must exit on its own once it sees the terminal status —
	// no context cancel needed.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("handler did not exit after terminal snapshot")
	}

	frame := w.body()
	if !strings.Contains(frame, `"status":"complete"`) {
		t.Errorf("snapshot frame missing status=complete: %q", frame)
	}
	if !strings.Contains(frame, `"wins":55`) {
		t.Errorf("snapshot frame missing wins=55: %q", frame)
	}
}

func TestHandleTournamentEvents_BroadcastClosesOnTerminal(t *testing.T) {
	sm := &Showmatch{
		gauntlets: map[string]*GauntletResult{
			"alice/korvold": {
				DeckKey: "alice/korvold",
				Status:  "running",
				Games:   10,
				Target:  500,
			},
		},
		gauntletSubs: map[string]map[*gauntletSubscriber]struct{}{},
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/tournaments/alice/korvold/events", nil)
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")

	w := newSSERecorder()
	done := make(chan struct{})
	go func() {
		sm.handleTournamentEvents(w, req)
		close(done)
	}()

	// Wait for the initial running snapshot so we know the subscriber
	// is registered before we broadcast.
	awaitSSEFrame(t, w, "snapshot", 2*time.Second)

	// Now publish a terminal snapshot via broadcastGauntlet. The
	// handler should write the second frame and exit.
	sm.broadcastGauntlet("alice/korvold", GauntletResult{
		DeckKey: "alice/korvold",
		Status:  "complete",
		Games:   500,
		Target:  500,
		Wins:    250,
		Losses:  250,
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("handler did not exit after terminal broadcast")
	}

	body := w.body()
	// Count how many "event: snapshot" lines we saw — must be at
	// least two (initial running snapshot + terminal complete).
	sc := bufio.NewScanner(strings.NewReader(body))
	frames := 0
	for sc.Scan() {
		if sc.Text() == "event: snapshot" {
			frames++
		}
	}
	if frames < 2 {
		t.Errorf("expected ≥2 snapshot frames (initial + terminal), got %d (body=%q)", frames, body)
	}
	if !strings.Contains(body, `"status":"complete"`) {
		t.Errorf("body missing terminal status=complete: %q", body)
	}
}
