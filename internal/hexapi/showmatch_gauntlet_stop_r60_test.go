package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// r60 — dead-run supersede + stop capability for gauntlets.
//
// Bug history: handleStartGauntlet returned any existing record whose
// Status == "running" verbatim. Nothing set a dead run's status off
// "running" (a single game panicking/hanging left the record stuck), so
// every future start on that deck returned the zombie record forever —
// the deck became permanently unrunnable, and there was no stop path.
//
// The fix adds a LastProgressAt heartbeat (refreshed as games complete),
// a staleness supersede in gauntletBlocksNewStart, and a stop endpoint
// that flips a running record to "stopped". These tests pin the
// supersede/keep decision (via the pure gauntletBlocksNewStart helper,
// which needs no deck-pool/credits/goroutine harness) and the stop
// endpoint's status flip + unblock.

// TestGauntletBlocksNewStart_StaleRunSuperseded — a "running" record
// whose last progress is older than the staleness window is treated as
// dead, so a new start is NOT blocked (it supersedes).
func TestGauntletBlocksNewStart_StaleRunSuperseded(t *testing.T) {
	now := time.Now()
	stale := &GauntletResult{
		DeckKey:        "alice/korvold",
		Status:         "running",
		StartedAt:      now.Add(-10 * time.Minute),
		LastProgressAt: now.Add(-(gauntletStaleAfter + 30*time.Second)),
	}
	if gauntletBlocksNewStart(stale, now) {
		t.Fatalf("stale running record should NOT block a new start (should supersede)")
	}
}

// TestGauntletBlocksNewStart_ActiveRunNotSuperseded — a "running" record
// that is still making progress (recent LastProgressAt) DOES block a new
// start, so a user can't spam concurrent runs on one deck.
func TestGauntletBlocksNewStart_ActiveRunNotSuperseded(t *testing.T) {
	now := time.Now()
	active := &GauntletResult{
		DeckKey:        "alice/korvold",
		Status:         "running",
		StartedAt:      now.Add(-5 * time.Minute),
		LastProgressAt: now.Add(-2 * time.Second), // game completed 2s ago
	}
	if !gauntletBlocksNewStart(active, now) {
		t.Fatalf("actively-progressing run should block a new start (return existing)")
	}

	// A just-launched run with no completed game yet (LastProgressAt
	// zero) must fall back to StartedAt and still be protected.
	fresh := &GauntletResult{
		DeckKey:   "alice/korvold",
		Status:    "running",
		StartedAt: now.Add(-1 * time.Second),
	}
	if !gauntletBlocksNewStart(fresh, now) {
		t.Fatalf("just-launched run (no heartbeat yet) should block a new start")
	}

	// Boundary: exactly at the threshold is still considered active
	// (the check is <= gauntletStaleAfter).
	edge := &GauntletResult{
		DeckKey:        "alice/korvold",
		Status:         "running",
		LastProgressAt: now.Add(-gauntletStaleAfter),
	}
	if !gauntletBlocksNewStart(edge, now) {
		t.Fatalf("run exactly at the staleness threshold should still be active")
	}

	// A running record with no timestamps at all (degenerate/hand-built)
	// must still block — we never supersede a run we can't prove is dead.
	noStamp := &GauntletResult{Status: "running", Games: 42}
	if !gauntletBlocksNewStart(noStamp, now) {
		t.Fatalf("running record with zero timestamps should block (cannot prove staleness)")
	}

	// Terminal statuses never block.
	for _, st := range []string{"complete", "stopped", "error"} {
		done := &GauntletResult{Status: st, LastProgressAt: now}
		if gauntletBlocksNewStart(done, now) {
			t.Errorf("status %q should never block a new start", st)
		}
	}
	if gauntletBlocksNewStart(nil, now) {
		t.Errorf("nil record should never block a new start")
	}
}

// TestHandleStopGauntlet_FlipsStatusAndUnblocks — the stop endpoint
// flips a running record to "stopped" and (crucially) that makes a
// subsequent start no longer blocked, i.e. the deck is runnable again.
func TestHandleStopGauntlet_FlipsStatusAndUnblocks(t *testing.T) {
	sm := &Showmatch{
		gauntlets: make(map[string]*GauntletResult),
	}
	deckKey := "alice/korvold"
	running := &GauntletResult{
		DeckKey:        deckKey,
		Status:         "running",
		StartedAt:      time.Now(),
		LastProgressAt: time.Now(), // actively progressing → currently blocks
	}
	sm.gauntlets[deckKey] = running

	// Precondition: an active run blocks a new start.
	if !gauntletBlocksNewStart(sm.gauntlets[deckKey], time.Now()) {
		t.Fatalf("precondition: active run should block a new start before stop")
	}

	req := httptest.NewRequest(http.MethodPost,
		"/api/gauntlet/"+url.PathEscape("alice")+"/"+url.PathEscape("korvold")+"/stop",
		strings.NewReader(""))
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")
	rr := httptest.NewRecorder()

	sm.handleStopGauntlet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v (raw=%q)", err, rr.Body.String())
	}
	if got, _ := body["status"].(string); got != "stopped" {
		t.Errorf("response status: want %q, got %q", "stopped", got)
	}

	// The stored record is flipped to "stopped" with StopRequested set
	// (so the runner goroutine breaks out on its next iteration).
	sm.gauntletMu.RLock()
	rec := sm.gauntlets[deckKey]
	sm.gauntletMu.RUnlock()
	if rec.Status != "stopped" {
		t.Errorf("stored status: want %q, got %q", "stopped", rec.Status)
	}
	if !rec.StopRequested {
		t.Errorf("StopRequested should be set after stop")
	}
	if rec.FinishedAt.IsZero() {
		t.Errorf("FinishedAt should be stamped after stop")
	}

	// The deck is now runnable again: the stopped record no longer
	// blocks a new start.
	if gauntletBlocksNewStart(rec, time.Now()) {
		t.Errorf("stopped record should NOT block a subsequent start")
	}
}

// TestHandleStopGauntlet_NoRunningRecord — stopping when nothing is
// running is a clean no-op with a "not_running" status, not an error.
func TestHandleStopGauntlet_NoRunningRecord(t *testing.T) {
	sm := &Showmatch{gauntlets: make(map[string]*GauntletResult)}

	// (a) no record at all
	req := httptest.NewRequest(http.MethodPost, "/api/gauntlet/alice/korvold/stop", strings.NewReader(""))
	req.SetPathValue("owner", "alice")
	req.SetPathValue("id", "korvold")
	rr := httptest.NewRecorder()
	sm.handleStopGauntlet(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("no-record: want 200, got %d", rr.Code)
	}
	var body map[string]any
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if got, _ := body["status"].(string); got != "not_running" {
		t.Errorf("no-record status: want %q, got %q", "not_running", got)
	}

	// (b) an already-complete record is not re-stopped.
	sm.gauntlets["alice/korvold"] = &GauntletResult{DeckKey: "alice/korvold", Status: "complete"}
	rr2 := httptest.NewRecorder()
	sm.handleStopGauntlet(rr2, req)
	var body2 map[string]any
	_ = json.NewDecoder(rr2.Body).Decode(&body2)
	if got, _ := body2["status"].(string); got != "not_running" {
		t.Errorf("complete-record status: want %q, got %q", "not_running", got)
	}
	sm.gauntletMu.RLock()
	if sm.gauntlets["alice/korvold"].Status != "complete" {
		t.Errorf("a complete record must not be flipped by stop")
	}
	sm.gauntletMu.RUnlock()
}

// TestHandleStopGauntlet_PathValidation — malformed owner/id are
// rejected before any state is touched (mirrors the start/get contract).
func TestHandleStopGauntlet_PathValidation(t *testing.T) {
	sm := &Showmatch{gauntlets: make(map[string]*GauntletResult)}
	req := httptest.NewRequest(http.MethodPost, "/api/gauntlet/../korvold/stop", strings.NewReader(""))
	req.SetPathValue("owner", "..")
	req.SetPathValue("id", "korvold")
	rr := httptest.NewRecorder()
	sm.handleStopGauntlet(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("path-traversal owner: want 400, got %d", rr.Code)
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode ErrorResponse: %v", err)
	}
	if body.Error.Message != "invalid owner or id" {
		t.Errorf("error message: want %q, got %q", "invalid owner or id", body.Error.Message)
	}
}
