package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stubPinger is a *sql.DB stand-in for ping tests. We can't easily
// fake *sql.DB itself (it's a concrete type with sealed internals),
// so the handler-level tests construct a real in-memory SQLite DB
// for the "reachable" path and a closed DB for the "unreachable"
// path.

// TestHealth_BasicResponseShape pins the JSON contract: status="ok",
// numeric uptime_sec, version string, boolean db_reachable. Defends
// against future refactors that drop a field or rename the casing —
// orchestrators have brittle field dependencies.
func TestHealth_BasicResponseShape(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", w.Code, w.Body.String())
	}
	var got healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v (raw=%q)", err, w.Body.String())
	}
	if got.Status != "ok" {
		t.Errorf("status: want ok, got %q", got.Status)
	}
	if got.UptimeSec < 0 {
		t.Errorf("uptime_sec must be non-negative, got %d", got.UptimeSec)
	}
	if got.Version == "" {
		t.Errorf("version must not be empty")
	}
}

// TestHealth_JSONFieldNames pins exact wire field names. A reviewer
// changing `json:"uptime_sec"` to `json:"uptimeSec"` would break
// every external monitor — this test catches it before review.
func TestHealth_JSONFieldNames(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
	wantKeys := []string{"status", "uptime_sec", "version", "db_reachable"}
	for _, k := range wantKeys {
		if _, ok := raw[k]; !ok {
			t.Errorf("missing key %q (have %v)", k, mapKeys(raw))
		}
	}
	// Defend against extra fields sneaking in — keeps the contract minimal.
	for k := range raw {
		known := false
		for _, want := range wantKeys {
			if k == want {
				known = true
				break
			}
		}
		if !known {
			t.Errorf("unexpected extra field %q in health response", k)
		}
	}
}

// TestHealth_NoDBReportsUnreachable confirms the boolean honestly
// reflects the absence of a DB rather than defaulting to true. A
// process running without persistence (dev / unit tests) must
// report db_reachable=false so a deploy that forgot to wire the DB
// connection string fails its readiness check loudly.
func TestHealth_NoDBReportsUnreachable(t *testing.T) {
	h := &Handler{} // no DB
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got healthResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.DBReachable {
		t.Fatal("nil DB must report db_reachable=false")
	}
}

// TestHealth_ReachableDBReportsTrue confirms a live DB is reflected
// correctly. Uses the in-memory SQLite driver via the existing test
// helper pattern (Ping is cheap and succeeds on a freshly-opened
// connection).
func TestHealth_ReachableDBReportsTrue(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("sqlite driver unavailable in this build: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		t.Skipf("sqlite ping failed: %v", err)
	}

	h := &Handler{}
	h.SetDB(db)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body parse: %v", err)
	}
	if !got.DBReachable {
		t.Fatal("live DB must report db_reachable=true")
	}
}

// TestHealth_ClosedDBReportsUnreachable confirms a closed/broken
// connection drops to false within the ping timeout — the
// orchestrator branch that needs strict DB readiness must be able
// to trust this signal.
func TestHealth_ClosedDBReportsUnreachable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Skipf("sqlite driver unavailable: %v", err)
	}
	_ = db.Close() // close BEFORE the request

	h := &Handler{}
	h.SetDB(db)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got healthResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.DBReachable {
		t.Fatal("closed DB must report db_reachable=false")
	}
	// Status stays "ok" even with DB down — see handleHealth docstring.
	if got.Status != "ok" {
		t.Errorf("status should remain ok with DB down (200 + db_reachable=false is the contract), got %q", got.Status)
	}
	if w.Code != http.StatusOK {
		t.Errorf("HTTP status should remain 200 with DB down, got %d", w.Code)
	}
}

// TestHealth_UptimeIncreases confirms uptime_sec actually grows
// across calls. Uses startedAt rewind to keep the test fast — we
// don't actually wait two seconds.
func TestHealth_UptimeIncreases(t *testing.T) {
	origStarted := startedAt
	startedAt = time.Now().Add(-30 * time.Second)
	defer func() { startedAt = origStarted }()

	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got healthResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.UptimeSec < 28 || got.UptimeSec > 35 {
		t.Errorf("uptime_sec for 30s-old process: want ~30, got %d", got.UptimeSec)
	}
}

// TestHealth_VersionOverride confirms the package var is honored —
// production builds use -ldflags to inject the real version, and
// this test makes sure the handler reads from the var rather than
// hardcoding "dev".
func TestHealth_VersionOverride(t *testing.T) {
	orig := Version
	Version = "r60-test-build"
	defer func() { Version = orig }()

	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var got healthResponse
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Version != "r60-test-build" {
		t.Fatalf("version override not honored: want r60-test-build, got %q", got.Version)
	}
}

// TestHealth_NoCacheHeader confirms the response carries
// Cache-Control: no-store. A monitor hitting /api/health every
// 10 seconds and getting a CDN-cached "all green" response from
// an hour ago would mask real outages — the no-store header
// blocks every intermediate cache.
func TestHealth_NoCacheHeader(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control: want no-store, got %q", got)
	}
}

// TestHealth_GETOnly confirms the route is GET-only. A POST would
// reach the mux but not match the GET-prefixed pattern — net/http
// mux returns 405 for method-mismatched but path-matched routes.
func TestHealth_GETOnly(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("POST", "/api/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Fatalf("POST should not match GET-only route, got 200")
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
