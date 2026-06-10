package hexapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// maintenance_mode_r60_test.go — pins HEXDEK_MAINTENANCE behavior: the
// gauntlet + spectate-spawn endpoints must return 503 with the maintenance
// message when the flag is on, so users see "down for maintenance" instead of
// kicking off games while the r61 engine fixes are in progress. The
// maintenance gate is the first statement in each handler, so a minimal
// Showmatch (no AST/oracle load) exercises exactly that path.

func TestMaintenanceMode_GauntletReturns503(t *testing.T) {
	sm := &Showmatch{maintenance: true}
	req := httptest.NewRequest("POST", "/api/gauntlet/owner/deck", nil)
	w := httptest.NewRecorder()
	sm.handleStartGauntlet(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("gauntlet in maintenance: got %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "maintenance") {
		t.Errorf("gauntlet body missing maintenance message: %s", w.Body.String())
	}
}

func TestMaintenanceMode_SpectateSpawnReturns503(t *testing.T) {
	sm := &Showmatch{maintenance: true}
	req := httptest.NewRequest("POST", "/api/spectate/spawn", strings.NewReader(`{"deck_id":"o/d"}`))
	w := httptest.NewRecorder()
	sm.handleSpawnSpectateRoom(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("spectate spawn in maintenance: got %d, want 503", w.Code)
	}
	if !strings.Contains(w.Body.String(), "maintenance") {
		t.Errorf("spectate body missing maintenance message: %s", w.Body.String())
	}
}

func TestMaintenanceMode_DefaultsOff(t *testing.T) {
	if (&Showmatch{}).maintenance {
		t.Error("maintenance must default to false (live) when HEXDEK_MAINTENANCE is unset")
	}
}
