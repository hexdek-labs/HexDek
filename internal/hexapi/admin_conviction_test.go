package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hexdek/hexdek/internal/hat"
)

func seedConviction(t *testing.T, n int) {
	t.Helper()
	hat.ResetConvictionTelemetry()
	for i := 1; i <= n; i++ {
		hat.TestingPushConvictionEvent(hat.ConvictionEvent{
			Seat:             i % 4,
			Turn:             i,
			RelativePosition: float64(-i) / 10,
			AnyTriggered:     i%3 == 0,
			ScoreTriggered:   i%3 == 0,
		})
	}
}

// TestAdminConviction_UnsetEnvFailsClosed: with no env-configured admin
// owner, the endpoint must refuse even a localhost Host header.
// (Pre-r60-audit: localhost Host header alone authenticated — a remote
// attacker could spoof it via the public reverse proxy or direct port
// hit on the LAN. See adminAnomalyAuth docstring for the CWE-290
// history.) The env-set + header-set happy path is exercised by the
// existing TestAdminConviction_AdminOwnerHeader.
func TestAdminConviction_UnsetEnvFailsClosed(t *testing.T) {
	t.Cleanup(hat.ResetConvictionTelemetry)
	seedConviction(t, 1)

	mux := http.NewServeMux()
	(&AdminConvictionHandler{}).Register(mux)

	req := httptest.NewRequest("GET", "http://localhost/api/admin/conviction-events", nil)
	req.Host = "localhost"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 (env unset, no fallback), got %d", rr.Code)
	}
}

func TestAdminConviction_RemoteForbidden(t *testing.T) {
	t.Cleanup(hat.ResetConvictionTelemetry)
	seedConviction(t, 1)

	mux := http.NewServeMux()
	(&AdminConvictionHandler{}).Register(mux)

	req := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/conviction-events", nil)
	req.Host = "hexdek.dev"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAdminConviction_AdminOwnerHeader(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	t.Cleanup(hat.ResetConvictionTelemetry)
	seedConviction(t, 2)

	mux := http.NewServeMux()
	(&AdminConvictionHandler{}).Register(mux)

	// Missing header → forbidden (even from localhost when env is set).
	req := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/conviction-events", nil)
	req.Host = "hexdek.dev"
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("missing header: got %d", rr.Code)
	}

	// Correct header → 200.
	req2 := httptest.NewRequest("GET", "http://hexdek.dev/api/admin/conviction-events", nil)
	req2.Host = "hexdek.dev"
	req2.Header.Set("X-HexDek-Owner", "alice")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("with header: got %d body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestAdminConviction_TriggeredFilter(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	t.Cleanup(hat.ResetConvictionTelemetry)
	seedConviction(t, 9) // 3 of them have AnyTriggered=true (turns 3, 6, 9)

	mux := http.NewServeMux()
	(&AdminConvictionHandler{}).Register(mux)

	req := httptest.NewRequest("GET", "http://localhost/api/admin/conviction-events?triggered=1", nil)
	req.Host = "localhost"
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	var resp struct {
		Count  int                   `json:"count"`
		Events []hat.ConvictionEvent `json:"events"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Count != 3 {
		t.Fatalf("triggered count=%d, want 3", resp.Count)
	}
	for _, e := range resp.Events {
		if !e.AnyTriggered {
			t.Errorf("non-triggered event leaked through filter: %+v", e)
		}
	}
}

func TestAdminConviction_SincePagination(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "alice")
	t.Cleanup(hat.ResetConvictionTelemetry)
	seedConviction(t, 5)

	mux := http.NewServeMux()
	(&AdminConvictionHandler{}).Register(mux)

	req := httptest.NewRequest("GET", "http://localhost/api/admin/conviction-events?since=3", nil)
	req.Host = "localhost"
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	var resp struct {
		Count  int                   `json:"count"`
		Events []hat.ConvictionEvent `json:"events"`
	}
	_ = json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Count != 2 {
		t.Fatalf("since=3 count=%d, want 2", resp.Count)
	}
	if resp.Events[0].Sequence != 4 || resp.Events[1].Sequence != 5 {
		t.Errorf("since=3 seqs = %d,%d; want 4,5", resp.Events[0].Sequence, resp.Events[1].Sequence)
	}
}
