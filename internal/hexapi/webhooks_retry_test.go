package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	_ "modernc.org/sqlite"
)

// webhooks_retry_test.go — coverage for the retry-with-backoff +
// dead-letter behaviour added on top of #303's single-shot
// dispatcher.
//
// Two layers under test:
//
//   1. The classifier + backoff helpers (pure functions; covered
//      by table-driven tests below).
//   2. The end-to-end dispatcher loop (real httptest server,
//      real SQLite, tests assert on captured request count +
//      webhooks row health + dead-letter row presence).
//
// All dispatcher tests narrow the retry policy to 1ms-scale via
// SetRetryPolicy so the backoff is exercised at test speed.

// openDispatcherDB returns an in-memory SQLite with BOTH schemas
// applied (webhooks + webhook_dead_letters), used by every retry
// test in this file.
func openDispatcherDB(t *testing.T) *sql.DB {
	t.Helper()
	d := openWebhookDB(t) // from webhooks_test.go — has webhooks schema
	if err := db.EnsureWebhookDeadLettersSchema(context.Background(), d); err != nil {
		t.Fatalf("EnsureWebhookDeadLettersSchema: %v", err)
	}
	return d
}

// -------------------------------------------------------------------
// Pure helpers
// -------------------------------------------------------------------

func TestClassifyDeliveryOutcome(t *testing.T) {
	cases := []struct {
		name   string
		status int
		err    error
		want   deliveryOutcome
	}{
		{"transport error → transient", 0, errors.New("dial: connection refused"), deliveryTransient},
		{"2xx → success", 200, nil, deliverySuccess},
		{"201 → success", 201, nil, deliverySuccess},
		{"204 → success", 204, nil, deliverySuccess},
		{"301 → success (treat redirects as ack)", 301, nil, deliverySuccess},
		{"400 → permanent (bad request)", 400, nil, deliveryPermanent},
		{"401 → permanent (auth bug)", 401, nil, deliveryPermanent},
		{"404 → permanent (URL gone)", 404, nil, deliveryPermanent},
		{"410 → permanent", 410, nil, deliveryPermanent},
		{"429 → transient (server asked us to wait)", 429, nil, deliveryTransient},
		{"500 → transient", 500, nil, deliveryTransient},
		{"503 → transient", 503, nil, deliveryTransient},
		{"504 → transient", 504, nil, deliveryTransient},
		{"100 (continue) → transient (degenerate, retry)", 100, nil, deliveryTransient},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyDeliveryOutcome(tc.status, tc.err); got != tc.want {
				t.Errorf("classify(%d, %v): want %v, got %v", tc.status, tc.err, tc.want, got)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{"  10 ", 10 * time.Second},
		{"-1", 0},
		{"0", 0},
		{"not a number", 0},
		{"Mon, 15 Nov 2027 14:00:00 GMT", 0}, // HTTP-date form intentionally not parsed
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			if got := parseRetryAfter(tc.in); got != tc.want {
				t.Errorf("parseRetryAfter(%q): want %v, got %v", tc.in, tc.want, got)
			}
		})
	}
}

func TestBackoffFor(t *testing.T) {
	// base=10ms, max=80ms → expected schedule: 10, 20, 40, 80, 80, 80...
	d := &WebhookDispatcher{
		backoffBase: 10 * time.Millisecond,
		backoffMax:  80 * time.Millisecond,
	}
	cases := []struct {
		next int
		want time.Duration
	}{
		{1, 10 * time.Millisecond}, // before second attempt
		{2, 20 * time.Millisecond},
		{3, 40 * time.Millisecond},
		{4, 80 * time.Millisecond}, // hit cap
		{5, 80 * time.Millisecond}, // capped
		{10, 80 * time.Millisecond},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(strconv.Itoa(tc.next), func(t *testing.T) {
			if got := d.backoffFor(tc.next); got != tc.want {
				t.Errorf("backoffFor(%d): want %v, got %v", tc.next, tc.want, got)
			}
		})
	}
}

func TestBackoffFor_ZeroBaseReturnsZero(t *testing.T) {
	d := &WebhookDispatcher{backoffBase: 0, backoffMax: time.Second}
	if got := d.backoffFor(5); got != 0 {
		t.Errorf("zero base: want 0, got %v", got)
	}
}

func TestSetRetryPolicy_Sanitisation(t *testing.T) {
	d := NewWebhookDispatcher(nil, nil)
	d.SetRetryPolicy(0, -1, -2)
	if d.maxAttempts != 1 {
		t.Errorf("maxAttempts clamped to 1: got %d", d.maxAttempts)
	}
	if d.backoffBase != 0 {
		t.Errorf("backoffBase normalized to 0: got %v", d.backoffBase)
	}
	if d.backoffMax != 0 {
		t.Errorf("backoffMax ≥ base: got %v", d.backoffMax)
	}
}

// -------------------------------------------------------------------
// retryServer: an httptest server that returns a programmable
// sequence of status codes (one per request).
// -------------------------------------------------------------------

type retryServer struct {
	t      *testing.T
	server *httptest.Server

	mu             sync.Mutex
	requests       int
	statuses       []int    // playback queue
	retryAfter     string   // sent on the first 429 / 503; empty disables
	bodySnapshots  [][]byte // captured request bodies
}

func newRetryServer(t *testing.T, statuses ...int) *retryServer {
	t.Helper()
	rs := &retryServer{t: t, statuses: statuses}
	rs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		_ = r.Body.Close()

		rs.mu.Lock()
		rs.requests++
		idx := rs.requests - 1
		if idx >= len(rs.statuses) {
			idx = len(rs.statuses) - 1
		}
		status := rs.statuses[idx]
		ra := rs.retryAfter
		rs.bodySnapshots = append(rs.bodySnapshots, body)
		rs.mu.Unlock()

		if (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable) && ra != "" {
			w.Header().Set("Retry-After", ra)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(rs.server.Close)
	return rs
}

func (rs *retryServer) count() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.requests
}

// -------------------------------------------------------------------
// Retry behaviour — happy paths
// -------------------------------------------------------------------

func TestDispatcher_RetryOn5xx_SucceedsBeforeExhaustion(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	rs := newRetryServer(t, 500, 500, 200)
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(5, time.Millisecond, 10*time.Millisecond)

	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{"x": 1})
	disp.Wait()

	if rs.count() != 3 {
		t.Errorf("want 3 HTTP attempts (500→500→200), got %d", rs.count())
	}
	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 0 {
		t.Errorf("success on retry should reset failures, got %d", w.Failures)
	}
	if w.LastStatus != 200 {
		t.Errorf("last_status: want 200, got %d", w.LastStatus)
	}
	n, _ := db.CountWebhookDeadLetters(ctx, d, id)
	if n != 0 {
		t.Errorf("success path must not write a dead-letter, got %d rows", n)
	}
}

func TestDispatcher_RetryOnTransportError_SucceedsOnReconnect(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	// A server that's down for the first two attempts, up for the
	// third. Simulated by closing then re-opening; simpler to use
	// an http.RoundTripper stub that flips after N attempts.
	var attempts atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := attempts.Add(1)
		if n < 3 {
			return nil, errors.New("simulated: connection refused")
		}
		// Third attempt: emit a 200.
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	client := &http.Client{Transport: transport}

	id, _ := db.InsertWebhook(ctx, d, "alice", "https://example.com/hook", WebhookEventGameEnd, "s")
	disp := NewWebhookDispatcher(d, client)
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(5, time.Millisecond, 10*time.Millisecond)
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{"x": 1})
	disp.Wait()

	if got := attempts.Load(); got != 3 {
		t.Errorf("want 3 attempts (2 errors + success), got %d", got)
	}
	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 0 {
		t.Errorf("transient recovery should reset failures, got %d", w.Failures)
	}
}

// -------------------------------------------------------------------
// Retry behaviour — exhaustion → dead-letter
// -------------------------------------------------------------------

func TestDispatcher_ExhaustedRetries_WritesDeadLetter(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	// Playback queue: server returns these statuses in order. With
	// maxAttempts=4 the dispatcher consumes the first four entries,
	// so the LAST attempt sees a 504 — that's what should land in
	// last_status + the dead-letter row.
	rs := newRetryServer(t, 500, 502, 503, 504) // always fail
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(4, time.Millisecond, 5*time.Millisecond)

	payload := map[string]any{"game_id": 7, "winner": 2}
	disp.Fire(ctx, WebhookEventGameEnd, payload)
	disp.Wait()

	if rs.count() != 4 {
		t.Errorf("want 4 attempts (= maxAttempts), got %d", rs.count())
	}

	// failures should bump exactly ONCE (one logical delivery
	// failed), not four times.
	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 1 {
		t.Errorf("failures: want 1 (per-delivery, not per-attempt), got %d", w.Failures)
	}
	if w.LastStatus != 504 {
		t.Errorf("last_status: want 504 (final attempt), got %d", w.LastStatus)
	}

	// Dead-letter row must exist with full payload + attempts=4.
	rows, _ := db.ListWebhookDeadLetters(ctx, d, id, 10)
	if len(rows) != 1 {
		t.Fatalf("want 1 dead-letter row, got %d", len(rows))
	}
	dl := rows[0]
	if dl.Attempts != 4 {
		t.Errorf("dl.Attempts: want 4, got %d", dl.Attempts)
	}
	if dl.LastStatus != 504 {
		t.Errorf("dl.LastStatus: want 504, got %d", dl.LastStatus)
	}
	if dl.EventType != WebhookEventGameEnd {
		t.Errorf("dl.EventType: want %q, got %q", WebhookEventGameEnd, dl.EventType)
	}
	// Payload round-trip: dispatcher marshals once, dead-letter
	// stores the same bytes verbatim.
	var got map[string]any
	if err := json.Unmarshal(dl.Payload, &got); err != nil {
		t.Fatalf("payload decode: %v (raw=%q)", err, dl.Payload)
	}
	if int(got["game_id"].(float64)) != 7 {
		t.Errorf("payload.game_id: want 7, got %v", got["game_id"])
	}
}

func TestDispatcher_Permanent4xx_DeadLettersImmediately(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	rs := newRetryServer(t, 404, 404, 404)
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(5, time.Millisecond, 10*time.Millisecond)
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()

	if rs.count() != 1 {
		t.Errorf("permanent 4xx should NOT retry: want 1 attempt, got %d", rs.count())
	}
	rows, _ := db.ListWebhookDeadLetters(ctx, d, id, 10)
	if len(rows) != 1 {
		t.Fatalf("want 1 dead-letter row (immediate on 4xx), got %d", len(rows))
	}
	if rows[0].LastStatus != 404 {
		t.Errorf("dl.LastStatus: want 404, got %d", rows[0].LastStatus)
	}
	if rows[0].Attempts != 1 {
		t.Errorf("dl.Attempts: want 1 (no retry on permanent), got %d", rows[0].Attempts)
	}
}

func TestDispatcher_429_IsRetriable(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	rs := newRetryServer(t, 429, 200)
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(3, time.Millisecond, 10*time.Millisecond)
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()

	if rs.count() != 2 {
		t.Errorf("want 2 attempts (429 → 200), got %d", rs.count())
	}
	rows, _ := db.ListWebhookDeadLetters(ctx, d, id, 10)
	if len(rows) != 0 {
		t.Errorf("successful retry should not write dead-letter: got %d rows", len(rows))
	}
}

func TestDispatcher_429_RetryAfter_IsHonored(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	rs := newRetryServer(t, 429, 200)
	rs.retryAfter = "1" // 1 second — way more than the default 1ms backoff
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")
	_ = id

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(3, time.Millisecond, 100*time.Millisecond)

	start := time.Now()
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	elapsed := time.Since(start)

	// Retry-After: 1 should dominate the 1ms backoff. backoffMax is
	// 100ms so the wait gets capped — we should see ≥ 100ms total.
	// Use a generous lower bound (90ms) to avoid scheduling flakes.
	if elapsed < 90*time.Millisecond {
		t.Errorf("Retry-After not honored: elapsed=%v, expected ≥ 90ms (capped at backoffMax=100ms)", elapsed)
	}
	if rs.count() != 2 {
		t.Errorf("want 2 attempts, got %d", rs.count())
	}
}

// -------------------------------------------------------------------
// Auto-disable still works with the new per-delivery counting
// -------------------------------------------------------------------

func TestDispatcher_FailuresCountOncePerDelivery_AutoDisableAtThreshold(t *testing.T) {
	d := openDispatcherDB(t)
	ctx := context.Background()
	// 9 × 500 — enough for 3 logical deliveries of 3 retries each.
	rs := newRetryServer(t, 500, 500, 500, 500, 500, 500, 500, 500, 500)
	id, _ := db.InsertWebhook(ctx, d, "alice", rs.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, rs.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetRetryPolicy(3, time.Millisecond, 5*time.Millisecond)
	disp.SetFailureThreshold(3) // disable after 3 *deliveries*, not 3 attempts

	// Fire three times — each should consume 3 attempts and bump
	// failures by 1 (since each delivery exhausts). After the
	// third Fire, failures == 3 and the row should be inactive.
	for i := 0; i < 3; i++ {
		disp.Fire(ctx, WebhookEventGameEnd, map[string]any{"i": i})
		disp.Wait()
	}

	if rs.count() != 9 {
		t.Errorf("want 9 HTTP attempts (3 deliveries × 3 retries), got %d", rs.count())
	}
	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 3 {
		t.Errorf("failures: want 3 (one per delivery), got %d", w.Failures)
	}
	if w.Active {
		t.Errorf("auto-disable should have flipped active=0 after 3 failed deliveries")
	}

	// And we should have 3 dead-letter rows — one per exhausted
	// delivery.
	n, _ := db.CountWebhookDeadLetters(ctx, d, id)
	if n != 3 {
		t.Errorf("dead-letter rows: want 3, got %d", n)
	}
}

// -------------------------------------------------------------------
// Dead-letter HTTP listing surface
// -------------------------------------------------------------------

func TestHandleListWebhookDeadLetters(t *testing.T) {
	h := &Handler{}
	d := openDispatcherDB(t)
	h.SetDB(d)
	ctx := context.Background()

	alice, _ := db.InsertWebhook(ctx, d, "alice", "https://a", WebhookEventGameEnd, "s")
	bob, _ := db.InsertWebhook(ctx, d, "bob", "https://b", WebhookEventGameEnd, "s")
	// Two dead-letter rows for alice's webhook, one for bob's.
	for i := 0; i < 2; i++ {
		_, _ = db.InsertWebhookDeadLetter(ctx, d, db.WebhookDeadLetter{
			WebhookID: alice, EventType: WebhookEventGameEnd, Payload: []byte("{}"),
			Attempts: 5, LastStatus: 500,
		})
	}
	_, _ = db.InsertWebhookDeadLetter(ctx, d, db.WebhookDeadLetter{
		WebhookID: bob, EventType: WebhookEventGameEnd, Payload: []byte("{}"),
		Attempts: 1, LastStatus: 404,
	})

	cases := []struct {
		name       string
		owner      string
		idStr      string
		wantStatus int
		wantCount  int
	}{
		{"happy path — alice lists her own", "alice", strconv.FormatInt(alice, 10), http.StatusOK, 2},
		{"other owner gets 404 (no existence leak)", "bob", strconv.FormatInt(alice, 10), http.StatusNotFound, 0},
		{"missing webhook 404", "alice", "9999", http.StatusNotFound, 0},
		{"invalid id 400", "alice", "abc", http.StatusBadRequest, 0},
		{"missing owner 401", "", strconv.FormatInt(alice, 10), http.StatusUnauthorized, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/webhooks/"+tc.idStr+"/dead-letters", nil)
			req.SetPathValue("id", tc.idStr)
			if tc.owner != "" {
				req.Header.Set("X-HexDek-Owner", tc.owner)
			}
			rr := httptest.NewRecorder()
			h.handleListWebhookDeadLetters(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			var resp struct {
				WebhookID   int64                    `json:"webhook_id"`
				DeadLetters []webhookDeadLetterEntry `json:"dead_letters"`
			}
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(resp.DeadLetters) != tc.wantCount {
				t.Errorf("count: want %d, got %d", tc.wantCount, len(resp.DeadLetters))
			}
			if resp.WebhookID != alice {
				t.Errorf("webhook_id: want %d, got %d", alice, resp.WebhookID)
			}
		})
	}
}

func TestHandleListWebhookDeadLetters_LimitParam(t *testing.T) {
	h := &Handler{}
	d := openDispatcherDB(t)
	h.SetDB(d)
	ctx := context.Background()
	id, _ := db.InsertWebhook(ctx, d, "alice", "https://a", WebhookEventGameEnd, "s")
	for i := 0; i < 10; i++ {
		_, _ = db.InsertWebhookDeadLetter(ctx, d, db.WebhookDeadLetter{
			WebhookID: id, EventType: WebhookEventGameEnd, Payload: []byte("{}"),
			Attempts: 1, LastStatus: 500,
		})
	}
	req := httptest.NewRequest(http.MethodGet,
		"/api/webhooks/"+strconv.FormatInt(id, 10)+"/dead-letters?limit=3", nil)
	req.SetPathValue("id", strconv.FormatInt(id, 10))
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	h.handleListWebhookDeadLetters(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	var resp struct {
		DeadLetters []webhookDeadLetterEntry `json:"dead_letters"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.DeadLetters) != 3 {
		t.Errorf("limit: want 3, got %d", len(resp.DeadLetters))
	}
}

func TestHandleListWebhookDeadLetters_NilDB(t *testing.T) {
	h := &Handler{} // no DB
	req := httptest.NewRequest(http.MethodGet, "/api/webhooks/1/dead-letters", nil)
	req.SetPathValue("id", "1")
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	h.handleListWebhookDeadLetters(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status: want 503, got %d", rr.Code)
	}
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

// roundTripFunc adapts a function to http.RoundTripper. Used by the
// transport-error retry test to swap a real connection with a
// programmable failure injector.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// suppress unused-import warning when strings only appears in
// trimming the Retry-After header.
var _ = strings.TrimSpace
