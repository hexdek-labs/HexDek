package hexapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	_ "modernc.org/sqlite"
)

// openWebhookDB returns an in-memory SQLite with the webhooks schema
// applied. Used by every test in this file so cleanup is uniform.
func openWebhookDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.EnsureWebhooksSchema(context.Background(), d); err != nil {
		t.Fatalf("EnsureWebhooksSchema: %v", err)
	}
	return d
}

// newWebhookHandler returns a *Handler with the webhooks DB wired in
// but no production deps (decks dir, cardDB, etc.) — the webhook
// surface doesn't depend on any of those.
func newWebhookHandler(t *testing.T) (*Handler, *sql.DB) {
	t.Helper()
	d := openWebhookDB(t)
	h := &Handler{}
	h.SetDB(d)
	return h, d
}

// -------------------------------------------------------------------
// URL validation (pure helper)
// -------------------------------------------------------------------

func TestValidateWebhookURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty rejected", "", true},
		{"https public OK", "https://example.com/hook", false},
		{"https with path OK", "https://example.com/path/x?q=1", false},
		{"http localhost OK", "http://localhost:9999/hook", false},
		{"http 127.0.0.1 OK", "http://127.0.0.1:8081/hook", false},
		{"http ::1 OK", "http://[::1]:9000/hook", false},
		{"http public rejected", "http://attacker.example.com/", true},
		{"ftp rejected", "ftp://example.com/file", true},
		{"file rejected", "file:///etc/passwd", true},
		{"missing scheme rejected", "example.com/x", true},
		{"missing host rejected", "https:///path", true},
		{"unparseable rejected", "://broken", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateWebhookURL(tc.url)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateWebhookURL(%q): err=%v, wantErr=%v", tc.url, err, tc.wantErr)
			}
		})
	}
}

// -------------------------------------------------------------------
// POST /api/webhooks — register
// -------------------------------------------------------------------

func TestHandleRegisterWebhook(t *testing.T) {
	type wantBody struct {
		err string
	}
	cases := []struct {
		name       string
		owner      string // X-HexDek-Owner; empty = header omitted
		body       string
		wantStatus int
		wantErr    string // exact ErrorResponse.Error when wantStatus != 201
	}{
		{
			name:       "happy path",
			owner:      "alice",
			body:       `{"url":"https://example.com/hook","event_type":"game.end"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing owner header rejected with 401",
			owner:      "",
			body:       `{"url":"https://example.com/hook","event_type":"game.end"}`,
			wantStatus: http.StatusUnauthorized,
			wantErr:    "missing X-HexDek-Owner header",
		},
		{
			name:       "invalid url rejected",
			owner:      "alice",
			body:       `{"url":"ftp://example.com/x","event_type":"game.end"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-localhost http rejected",
			owner:      "alice",
			body:       `{"url":"http://attacker.example.com/","event_type":"game.end"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unsupported event rejected",
			owner:      "alice",
			body:       `{"url":"https://example.com/","event_type":"player.win"}`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "unsupported event_type",
		},
		{
			name:       "malformed JSON rejected",
			owner:      "alice",
			body:       `{not-json`,
			wantStatus: http.StatusBadRequest,
			wantErr:    "invalid JSON body",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newWebhookHandler(t)
			req := httptest.NewRequest(http.MethodPost, "/api/webhooks", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			if tc.owner != "" {
				req.Header.Set("X-HexDek-Owner", tc.owner)
			}
			rr := httptest.NewRecorder()
			h.handleRegisterWebhook(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantStatus == http.StatusCreated {
				var resp webhookRegisterResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode register response: %v (raw=%q)", err, rr.Body.String())
				}
				if resp.ID <= 0 {
					t.Errorf("id: want > 0, got %d", resp.ID)
				}
				if len(resp.Secret) < 32 {
					t.Errorf("secret too short: %q", resp.Secret)
				}
				if resp.EventType != "game.end" {
					t.Errorf("event_type: want game.end, got %q", resp.EventType)
				}
				return
			}
			// Error path — verify the unified envelope.
			var body ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
			}
			if body.Status != tc.wantStatus {
				t.Errorf("body.status: want %d, got %d", tc.wantStatus, body.Status)
			}
			if tc.wantErr != "" && body.Error != tc.wantErr {
				t.Errorf("body.error: want %q, got %q", tc.wantErr, body.Error)
			}
		})
	}
}

func TestHandleRegisterWebhook_NilDB(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks",
		strings.NewReader(`{"url":"https://e","event_type":"game.end"}`))
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	h.handleRegisterWebhook(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: want 503, got %d", rr.Code)
	}
}

// -------------------------------------------------------------------
// GET /api/webhooks — list (omits secret)
// -------------------------------------------------------------------

func TestHandleListWebhooks(t *testing.T) {
	h, d := newWebhookHandler(t)
	ctx := context.Background()

	// Seed: 2 alice + 1 bob.
	id1, _ := db.InsertWebhook(ctx, d, "alice", "https://a/1", "game.end", "alice-secret-1")
	_, _ = db.InsertWebhook(ctx, d, "bob", "https://b/1", "game.end", "bob-secret")
	id2, _ := db.InsertWebhook(ctx, d, "alice", "https://a/2", "game.end", "alice-secret-2")

	req := httptest.NewRequest(http.MethodGet, "/api/webhooks", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	h.handleListWebhooks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp struct {
		Owner    string             `json:"owner"`
		Webhooks []webhookListEntry `json:"webhooks"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
	}
	if resp.Owner != "alice" {
		t.Errorf("owner: want alice, got %q", resp.Owner)
	}
	if len(resp.Webhooks) != 2 {
		t.Fatalf("want 2 webhooks for alice, got %d", len(resp.Webhooks))
	}
	// Newest first.
	if resp.Webhooks[0].ID != id2 || resp.Webhooks[1].ID != id1 {
		t.Errorf("order: want [%d, %d], got [%d, %d]",
			id2, id1, resp.Webhooks[0].ID, resp.Webhooks[1].ID)
	}

	// Critical: response body must NEVER contain the secret.
	raw := rr.Body.Bytes()
	if strings.Contains(string(raw), "alice-secret") || strings.Contains(string(raw), "bob-secret") {
		t.Errorf("list response leaked a secret: %s", string(raw))
	}
}

func TestHandleListWebhooks_RequiresOwner(t *testing.T) {
	h, _ := newWebhookHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/webhooks", nil)
	rr := httptest.NewRecorder()
	h.handleListWebhooks(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: want 401, got %d", rr.Code)
	}
}

// -------------------------------------------------------------------
// DELETE /api/webhooks/{id}
// -------------------------------------------------------------------

func TestHandleDeleteWebhook(t *testing.T) {
	h, d := newWebhookHandler(t)
	ctx := context.Background()
	id, _ := db.InsertWebhook(ctx, d, "alice", "https://a", "game.end", "s")

	cases := []struct {
		name       string
		owner      string
		idStr      string
		wantStatus int
	}{
		{"happy path — own webhook", "alice", strconv.FormatInt(id, 10), http.StatusOK},
		{"other owner gets 404 (does not leak existence)", "bob", strconv.FormatInt(id, 10), http.StatusNotFound},
		{"missing webhook 404", "alice", "9999", http.StatusNotFound},
		{"invalid id 400", "alice", "abc", http.StatusBadRequest},
		{"zero id 400", "alice", "0", http.StatusBadRequest},
		{"missing owner 401", "", "1", http.StatusUnauthorized},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/api/webhooks/"+tc.idStr, nil)
			req.SetPathValue("id", tc.idStr)
			if tc.owner != "" {
				req.Header.Set("X-HexDek-Owner", tc.owner)
			}
			rr := httptest.NewRecorder()
			h.handleDeleteWebhook(rr, req)
			if rr.Code != tc.wantStatus {
				t.Errorf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
		})
	}

	// Sanity: after the happy-path subtest succeeded, the row is gone.
	if _, err := db.GetWebhook(ctx, d, id); err == nil {
		t.Errorf("row %d should have been deleted by the happy-path subtest", id)
	}
}

// -------------------------------------------------------------------
// Dispatcher: Fire + signature verification + auto-disable
// -------------------------------------------------------------------

// captureServer is a tiny test HTTP server that records every
// inbound request body + headers and lets the test step through
// signature / event-type assertions.
type captureServer struct {
	t        *testing.T
	server   *httptest.Server
	mu       sync.Mutex
	requests []capturedRequest

	// nextStatus is the status code each handler invocation returns.
	// Atomic so tests that flip it concurrently (e.g. simulate
	// recovery) don't race.
	nextStatus atomic.Int64
}

type capturedRequest struct {
	URL       string
	Body      []byte
	Event     string
	Signature string
	Timestamp string
	WebhookID string
}

func newCaptureServer(t *testing.T, initialStatus int) *captureServer {
	t.Helper()
	c := &captureServer{t: t}
	c.nextStatus.Store(int64(initialStatus))
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		c.mu.Lock()
		c.requests = append(c.requests, capturedRequest{
			URL:       r.URL.Path,
			Body:      body,
			Event:     r.Header.Get("X-HexDek-Event"),
			Signature: r.Header.Get("X-HexDek-Signature"),
			Timestamp: r.Header.Get("X-HexDek-Timestamp"),
			WebhookID: r.Header.Get("X-HexDek-Webhook-Id"),
		})
		c.mu.Unlock()
		w.WriteHeader(int(c.nextStatus.Load()))
	}))
	t.Cleanup(c.server.Close)
	return c
}

func (c *captureServer) snapshot() []capturedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedRequest, len(c.requests))
	copy(out, c.requests)
	return out
}

func TestDispatcher_FireGameEnd_SignsPayloadAndDelivers(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	secret := "test-secret-key"
	cap := newCaptureServer(t, http.StatusOK)
	id, _ := db.InsertWebhook(ctx, d, "alice", cap.server.URL+"/hook", WebhookEventGameEnd, secret)

	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})

	game := CompletedGame{
		GameID:     42,
		Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
		DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
		Winner:     2,
		WinnerName: "Muldrotha",
		Turns:      14,
		EndReason:  "lethal_combat_damage",
		FinishedAt: time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC),
	}
	disp.FireGameEnd(ctx, game)
	disp.Wait()

	reqs := cap.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("want 1 delivery, got %d", len(reqs))
	}
	got := reqs[0]
	if got.URL != "/hook" {
		t.Errorf("URL: want /hook, got %q", got.URL)
	}
	if got.Event != WebhookEventGameEnd {
		t.Errorf("event header: want %q, got %q", WebhookEventGameEnd, got.Event)
	}
	if got.WebhookID != strconv.FormatInt(id, 10) {
		t.Errorf("webhook id header: want %d, got %q", id, got.WebhookID)
	}
	if got.Timestamp == "" {
		t.Errorf("timestamp header missing")
	}

	// Verify signature: HMAC-SHA256(secret, body), hex-prefixed with sha256=.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(got.Body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got.Signature != want {
		t.Errorf("signature mismatch:\n want %s\n  got %s", want, got.Signature)
	}

	// Body payload sanity.
	var payload map[string]any
	if err := json.Unmarshal(got.Body, &payload); err != nil {
		t.Fatalf("body not JSON: %v (%q)", err, got.Body)
	}
	if payload["event"] != WebhookEventGameEnd {
		t.Errorf("payload.event: want %q, got %v", WebhookEventGameEnd, payload["event"])
	}
	if f, _ := payload["game_id"].(float64); int(f) != 42 {
		t.Errorf("payload.game_id: want 42, got %v", payload["game_id"])
	}
	if payload["winner_name"] != "Muldrotha" {
		t.Errorf("payload.winner_name: want Muldrotha, got %v", payload["winner_name"])
	}

	// Health was recorded.
	w, _ := db.GetWebhook(ctx, d, id)
	if w.LastStatus != 200 {
		t.Errorf("last_status: want 200, got %d", w.LastStatus)
	}
	if w.Failures != 0 {
		t.Errorf("failures after 200: want 0, got %d", w.Failures)
	}
}

func TestDispatcher_Fire_SkipsInactiveAndOtherEventTypes(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	cap := newCaptureServer(t, http.StatusOK)
	// Three webhooks: one for game.end, one for a different event,
	// one for game.end but inactive.
	idActive, _ := db.InsertWebhook(ctx, d, "alice", cap.server.URL+"/active", WebhookEventGameEnd, "s1")
	_, _ = db.InsertWebhook(ctx, d, "alice", cap.server.URL+"/other", "some.other.event", "s2")
	idInactive, _ := db.InsertWebhook(ctx, d, "alice", cap.server.URL+"/inactive", WebhookEventGameEnd, "s3")
	_ = db.SetWebhookActive(ctx, d, idInactive, false)

	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{"foo": "bar"})
	disp.Wait()

	reqs := cap.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("want 1 delivery (active game.end only), got %d", len(reqs))
	}
	if reqs[0].URL != "/active" {
		t.Errorf("delivered to wrong url: %s", reqs[0].URL)
	}
	if reqs[0].WebhookID != strconv.FormatInt(idActive, 10) {
		t.Errorf("webhook id: want %d, got %s", idActive, reqs[0].WebhookID)
	}
}

func TestDispatcher_Fire_5xxBumpsFailuresAndAutoDisables(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	cap := newCaptureServer(t, http.StatusInternalServerError)
	id, _ := db.InsertWebhook(ctx, d, "alice", cap.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetFailureThreshold(2) // disable after 2 consecutive 5xx

	// First 5xx — still active.
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	w, _ := db.GetWebhook(ctx, d, id)
	if !w.Active {
		t.Errorf("after 1 failure should still be active")
	}
	if w.Failures != 1 || w.LastStatus != 500 {
		t.Errorf("after 1 failure: failures=%d last_status=%d", w.Failures, w.LastStatus)
	}

	// Second 5xx — auto-disable.
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	w, _ = db.GetWebhook(ctx, d, id)
	if w.Active {
		t.Errorf("after 2 failures should be inactive")
	}
	if w.Failures != 2 {
		t.Errorf("failures: want 2, got %d", w.Failures)
	}

	// Third Fire — webhook is inactive so no new delivery is attempted.
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	if got := len(cap.snapshot()); got != 2 {
		t.Errorf("third Fire should be skipped (inactive); got %d total deliveries", got)
	}
}

func TestDispatcher_Fire_RecoveryResetsFailures(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	cap := newCaptureServer(t, http.StatusInternalServerError)
	id, _ := db.InsertWebhook(ctx, d, "alice", cap.server.URL, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.SetFailureThreshold(5)

	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 1 {
		t.Fatalf("setup: want failures=1, got %d", w.Failures)
	}

	// Now flip the server to 200 and re-fire.
	cap.nextStatus.Store(int64(http.StatusOK))
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()
	w, _ = db.GetWebhook(ctx, d, id)
	if w.Failures != 0 {
		t.Errorf("after 200 should reset to 0, got %d", w.Failures)
	}
	if w.LastStatus != 200 {
		t.Errorf("last_status: want 200, got %d", w.LastStatus)
	}
}

func TestDispatcher_Fire_TransportErrorCountsAsFailure(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	// Pick a URL that will fail to dial — use an unused port via
	// httptest then immediately close so the next Do() gets ECONNREFUSED.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	u := srv.URL
	srv.Close()

	id, _ := db.InsertWebhook(ctx, d, "alice", u, WebhookEventGameEnd, "s")

	disp := NewWebhookDispatcher(d, &http.Client{Timeout: 500 * time.Millisecond})
	disp.SetLogf(func(string, ...any) {})
	disp.SetFailureThreshold(0) // never auto-disable so we can inspect failures
	disp.Fire(ctx, WebhookEventGameEnd, map[string]any{})
	disp.Wait()

	w, _ := db.GetWebhook(ctx, d, id)
	if w.Failures != 1 {
		t.Errorf("transport error should bump failures: got %d", w.Failures)
	}
	if w.LastStatus != -1 {
		t.Errorf("transport error should set last_status=-1, got %d", w.LastStatus)
	}
}

func TestDispatcher_Fire_UnsupportedEvent_IsNoOp(t *testing.T) {
	d := openWebhookDB(t)
	ctx := context.Background()
	cap := newCaptureServer(t, http.StatusOK)
	_, _ = db.InsertWebhook(ctx, d, "alice", cap.server.URL, "made.up.event", "s")

	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.Fire(ctx, "made.up.event", map[string]any{})
	disp.Wait()

	if got := len(cap.snapshot()); got != 0 {
		t.Errorf("unsupported event should not fire: %d deliveries", got)
	}
}

// -------------------------------------------------------------------
// End-to-end: register via HTTP, fire via dispatcher, receiver gets it.
// -------------------------------------------------------------------

func TestWebhooks_EndToEnd_RegisterAndFire(t *testing.T) {
	h, d := newWebhookHandler(t)
	cap := newCaptureServer(t, http.StatusOK)

	// Register via the HTTP layer.
	body := `{"url":"` + cap.server.URL + `/hook","event_type":"game.end"}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhooks", strings.NewReader(body))
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	h.handleRegisterWebhook(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("register: want 201, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var resp webhookRegisterResponse
	_ = json.NewDecoder(rr.Body).Decode(&resp)

	// Fire via the dispatcher (production path).
	disp := NewWebhookDispatcher(d, cap.server.Client())
	disp.SetLogf(func(string, ...any) {})
	disp.FireGameEnd(context.Background(), CompletedGame{
		GameID: 7, Winner: 0, WinnerName: "Korvold",
		Commanders: []string{"Korvold", "Atraxa", "Muldrotha", "Breya"},
		DeckKeys:   []string{"alice/k", "bob/a", "carol/m", "dave/b"},
		Turns:      11, EndReason: "lethal", FinishedAt: time.Now(),
	})
	disp.Wait()

	reqs := cap.snapshot()
	if len(reqs) != 1 {
		t.Fatalf("want 1 delivery, got %d", len(reqs))
	}
	// Signature verifies against the secret we just received.
	mac := hmac.New(sha256.New, []byte(resp.Secret))
	mac.Write(reqs[0].Body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if reqs[0].Signature != want {
		t.Errorf("signature: want %s, got %s", want, reqs[0].Signature)
	}

	// Now deregister via HTTP and confirm a second Fire is silent.
	delReq := httptest.NewRequest(http.MethodDelete,
		"/api/webhooks/"+strconv.FormatInt(resp.ID, 10), nil)
	delReq.SetPathValue("id", strconv.FormatInt(resp.ID, 10))
	delReq.Header.Set("X-HexDek-Owner", "alice")
	delRR := httptest.NewRecorder()
	h.handleDeleteWebhook(delRR, delReq)
	if delRR.Code != http.StatusOK {
		t.Fatalf("delete: want 200, got %d", delRR.Code)
	}

	disp.FireGameEnd(context.Background(), CompletedGame{GameID: 8})
	disp.Wait()
	if got := len(cap.snapshot()); got != 1 {
		t.Errorf("after delete, fire should be silent; got %d total deliveries", got)
	}
}

// Compile-time pin: webhookListEntry must omit the secret field. The
// list endpoint depends on this contract — we don't have JSON
// tag inspection, but if anyone adds a Secret field with a JSON tag
// the round-trip test in TestHandleListWebhooks will catch the
// substring "alice-secret" leaking into the body.
var _ webhookListEntry

// Compile-time pin: validateWebhookURL is a private helper. Using
// url.Parse here for any future maintainer who wants to extend the
// validation — we already use it inside the helper; this no-op
// import-check keeps the IDE from suggesting we drop net/url.
var _ = url.Parse
