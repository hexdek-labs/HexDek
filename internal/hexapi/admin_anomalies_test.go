package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/hexdek/hexdek/internal/anticheat"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, "audit.db") + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// seedFlags writes one synthetic flag so list/resolve handlers have
// something to operate on without depending on the audit math (which
// is tested directly in internal/anticheat).
func seedFlag(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec(`
		INSERT INTO contributor_flags
		  (contributor_id, metric, metric_value, pop_mean, pop_stddev, z_score, severity, detected_at)
		VALUES ('cheater', 'win_rate', 0.95, 0.25, 0.06, 11.6, 1, 1700000000)`)
	if err != nil {
		t.Fatalf("seed flag: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestHandleListAnomalies_UnsetEnvFailsClosed pins the post-r60-audit
// contract: no HEXDEK_ADMIN_OWNER configured → no admin access, period.
// (Pre-fix behavior fell back to "Host == localhost" which a remote
// attacker could spoof — see TestHandleListAnomalies_SpoofedHostHeaderRejected
// below.)
func TestHandleListAnomalies_UnsetEnvFailsClosed(t *testing.T) {
	db := newTestDB(t)
	auditor, err := anticheat.NewStatisticalAuditor(db)
	if err != nil {
		t.Fatalf("auditor: %v", err)
	}
	seedFlag(t, db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	// Localhost host header, no env, no header — pre-fix: 200. Post-fix: 403.
	req := httptest.NewRequest("GET", "http://localhost/api/admin/anomalies", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d (body=%s) — want 403, HEXDEK_ADMIN_OWNER must be required",
			rr.Code, rr.Body.String())
	}
}

// TestHandleListAnomalies_SpoofedHostHeaderRejected is the explicit
// CWE-290 regression for the dropped Host-header fallback. A remote
// attacker who reaches the server (LAN, WireGuard, or through a reverse
// proxy that preserves Host) used to be able to send `Host: localhost`
// and get full admin access. With the env var SET (production shape)
// the spoofed host must not affect the auth decision.
func TestHandleListAnomalies_SpoofedHostHeaderRejected(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)
	seedFlag(t, db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	// Attacker spoofs every localhost-shaped Host header but doesn't know
	// the admin owner slug. Each MUST 403.
	for _, host := range []string{"localhost", "127.0.0.1", "[::1]", "localhost:8090"} {
		req := httptest.NewRequest("GET", "http://attacker.example/api/admin/anomalies", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("spoofed Host=%q: status=%d, want 403 (admin owner header missing)", host, rr.Code)
		}
	}
}

func TestHandleListAnomalies_RemoteRequiresAdminOwner(t *testing.T) {
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	// No HEXDEK_ADMIN_OWNER set, non-localhost Host → forbidden.
	req := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/anomalies", nil)
	req.Host = "hexdek.dev"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-localhost without admin owner, got %d", rr.Code)
	}
}

func TestHandleListAnomalies_AdminOwnerEnvAllowed(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)
	seedFlag(t, db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	// Without header → forbidden even on localhost when env is set.
	req := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/anomalies", nil)
	req.Host = "hexdek.dev"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 without admin header, got %d", rr.Code)
	}

	// With matching header → ok.
	req2 := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/anomalies", nil)
	req2.Host = "hexdek.dev"
	req2.Header.Set("X-HexDek-Owner", "alice")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("expected 200 with admin header, got %d (body=%s)", rr2.Code, rr2.Body.String())
	}
}

func TestHandleResolveAnomaly_HappyPath(t *testing.T) {
	// Post-r60-audit: admin endpoints require env-configured owner +
	// matching header. No more Host-header fallback.
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)
	id := seedFlag(t, db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	body := strings.NewReader(`{"note":"reviewed — false positive"}`)
	req := httptest.NewRequest("POST", "http://localhost/api/admin/anomalies/"+strconv.FormatInt(id, 10)+"/resolve", body)
	req.Host = "localhost"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	// Verify the resolution landed in SQLite.
	flags, _ := auditor.ListFlags(context.Background(), false, 0)
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag in history, got %d", len(flags))
	}
	if flags[0].ResolvedAt == nil {
		t.Errorf("ResolvedAt should be set")
	}
	if flags[0].ResolvedBy != "alice" {
		t.Errorf("ResolvedBy = %q, want alice", flags[0].ResolvedBy)
	}
	if !strings.Contains(flags[0].ResolvedNote, "false positive") {
		t.Errorf("ResolvedNote = %q, want note text", flags[0].ResolvedNote)
	}

	// Re-resolving the same flag should 404 — append-only audit history.
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "http://localhost/api/admin/anomalies/"+strconv.FormatInt(id, 10)+"/resolve", strings.NewReader(`{}`))
	req2.Host = "localhost"
	req2.Header.Set("X-HexDek-Owner", "alice")
	mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("expected 404 on re-resolve, got %d", rr2.Code)
	}
}

func TestHandleResolveAnomaly_BadID(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	for _, idStr := range []string{"abc", "0", "-1"} {
		req := httptest.NewRequest("POST", "http://localhost/api/admin/anomalies/"+idStr+"/resolve", strings.NewReader(`{}`))
		req.Host = "localhost"
		req.Header.Set("X-HexDek-Owner", "alice")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Errorf("id=%q: expected 400, got %d", idStr, rr.Code)
		}
	}
}

func TestHandleListAnomalies_OnlyActiveFilter(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	db := newTestDB(t)
	auditor, _ := anticheat.NewStatisticalAuditor(db)
	id := seedFlag(t, db)
	// Seed a second flag and resolve the first.
	seedFlag(t, db)
	if err := auditor.ResolveFlag(context.Background(), id, "alice", "ok"); err != nil {
		t.Fatalf("resolve: %v", err)
	}

	mux := http.NewServeMux()
	RegisterAdminAnomalies(mux, auditor)

	// Default — only_active=true.
	req := httptest.NewRequest("GET", "http://localhost/api/admin/anomalies", nil)
	req.Host = "localhost"
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	var resp struct{ Count int `json:"count"` }
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Count != 1 {
		t.Errorf("default (only_active) count = %d, want 1", resp.Count)
	}

	// include_resolved=1 → both rows.
	req2 := httptest.NewRequest("GET", "http://localhost/api/admin/anomalies?include_resolved=1", nil)
	req2.Host = "localhost"
	req2.Header.Set("X-HexDek-Owner", "alice")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	var resp2 struct{ Count int `json:"count"` }
	if err := json.NewDecoder(rr2.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp2.Count != 2 {
		t.Errorf("include_resolved count = %d, want 2", resp2.Count)
	}
}
