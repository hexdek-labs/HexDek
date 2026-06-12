package hexapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	hexdb "github.com/hexdek/hexdek/internal/db"
)

// Regression pin for POST /api/telemetry/error — the first-party sink
// for the frontend error instrumentation (r61). Asserts the happy path
// inserts a row, anon_id is optional (iOS-private-mode sessions have
// none), malformed anon_ids degrade to NULL instead of rejecting the
// report, field clamps hold, and an empty message 400s.
func TestErrorTelemetryEndpoint(t *testing.T) {
	tmp := t.TempDir()
	db, err := hexdb.Open(tmp + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db}
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/api/telemetry/error", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("post: %v", err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	const anon = "11111111-2222-3333-4444-555555555555"

	// Happy path: boundary-caught render crash with anon id.
	if code := post(`{"anon_id":"` + anon + `","message":"ReferenceError: pls is not defined","stack":"at DeckArchive","kind":"render","source":"boundary","path":"/decks/o/d","timestamp":1700000000000}`); code != 204 {
		t.Errorf("happy path: status %d (want 204)", code)
	}
	// No anon id (private mode) — still accepted.
	if code := post(`{"message":"SecurityError: storage blocked","kind":"storage","source":"boundary"}`); code != 204 {
		t.Errorf("no anon id: status %d (want 204)", code)
	}
	// Malformed anon id — accepted, stored as NULL (don't drop the error).
	if code := post(`{"anon_id":"not-a-uuid","message":"TypeError: x is undefined","kind":"uncaught","source":"window"}`); code != 204 {
		t.Errorf("bad anon id: status %d (want 204)", code)
	}
	// Empty message — rejected.
	if code := post(`{"anon_id":"` + anon + `","message":"   "}`); code != 400 {
		t.Errorf("empty message: status %d (want 400)", code)
	}
	// Garbage body — rejected.
	if code := post(`{not json`); code != 400 {
		t.Errorf("garbage body: status %d (want 400)", code)
	}

	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM frontend_errors`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Errorf("frontend_errors rows = %d (want 3)", n)
	}

	// Row content: the happy-path row keeps its anon id; the malformed
	// one is NULL.
	var gotAnon *string
	var msg, kind, source, path string
	var ts int64
	if err := db.QueryRow(
		`SELECT anon_id, message, kind, source, path, ts FROM frontend_errors WHERE message LIKE 'ReferenceError%'`).
		Scan(&gotAnon, &msg, &kind, &source, &path, &ts); err != nil {
		t.Fatalf("select happy row: %v", err)
	}
	if gotAnon == nil || *gotAnon != anon {
		t.Errorf("anon_id = %v (want %s)", gotAnon, anon)
	}
	if kind != "render" || source != "boundary" || path != "/decks/o/d" || ts != 1700000000000 {
		t.Errorf("row fields = kind:%s source:%s path:%s ts:%d", kind, source, path, ts)
	}
	var nullAnon int
	if err := db.QueryRow(`SELECT COUNT(*) FROM frontend_errors WHERE anon_id IS NULL`).Scan(&nullAnon); err != nil {
		t.Fatalf("count null anon: %v", err)
	}
	if nullAnon != 2 {
		t.Errorf("NULL anon_id rows = %d (want 2: private-mode + malformed)", nullAnon)
	}
}

// The per-IP limiter clips a crash loop but lets a real burst through.
func TestErrorTelemetryRateLimit(t *testing.T) {
	tmp := t.TempDir()
	db, err := hexdb.Open(tmp + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	h := &Handler{db: db, ErrorTelemetryLimiter: NewRateLimiter(3, 1.0/60.0)}
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	codes := make([]int, 0, 5)
	for i := 0; i < 5; i++ {
		resp, err := http.Post(srv.URL+"/api/telemetry/error", "application/json",
			bytes.NewBufferString(`{"message":"crash loop tick"}`))
		if err != nil {
			t.Fatalf("post %d: %v", i, err)
		}
		resp.Body.Close()
		codes = append(codes, resp.StatusCode)
	}
	for i := 0; i < 3; i++ {
		if codes[i] != 204 {
			t.Errorf("burst request %d: status %d (want 204)", i, codes[i])
		}
	}
	for i := 3; i < 5; i++ {
		if codes[i] != http.StatusTooManyRequests {
			t.Errorf("over-budget request %d: status %d (want 429)", i, codes[i])
		}
	}
}
