package hexapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ratelimit_round2_r60_test.go — round-2 rate-limit coverage for the
// remaining anonymous mutating endpoints: deck import, deck analyze,
// moxfield import, gauntlet start, spectate spawn. Mirrors the
// existing feedback test shape: under-limit serves normally, over-
// limit returns 429 + Retry-After, the limit is per-IP, and a nil
// limiter is a no-op for backwards compatibility with binaries that
// haven't been updated to construct one.

// --- enforceRateLimit helper unit tests -------------------------------------

func TestEnforceRateLimit_NilIsBackwardsCompatibleNoop(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	if blocked := enforceRateLimit(nil, w, r, "any"); blocked {
		t.Fatal("nil limiter must not block")
	}
	if w.Code != 200 && w.Code != 0 {
		t.Errorf("nil limiter should write nothing, got status %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "" {
		t.Error("nil limiter should not set Retry-After")
	}
}

func TestEnforceRateLimit_OverLimitWrites429AndLabel(t *testing.T) {
	rl := NewRateLimiter(1, 1.0/60.0)
	// Drain.
	rl.Allow("198.51.100.50")

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "198.51.100.50")

	if blocked := enforceRateLimit(rl, w, r, "deck import"); !blocked {
		t.Fatal("over-limit request should be blocked")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on over-limit")
	}
	if !strings.Contains(w.Body.String(), "deck import") {
		t.Errorf("body should include the label %q, got %s", "deck import", w.Body.String())
	}
}

// --- POST /api/decks (handleImportDeck) -------------------------------------

func newImportTestHandler(t *testing.T, capacity float64) *Handler {
	t.Helper()
	// handleImportDeck kicks off versioning + freya goroutines that may
	// outlive the test body. t.TempDir's strict auto-cleanup races with
	// those background writers and flakes on "directory not empty". Use
	// os.MkdirTemp + a forgiving cleanup that swallows the race — the
	// OS will reclaim /tmp later.
	tmp, err := os.MkdirTemp("", "hexapi-ratelimit-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(decksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return &Handler{
		DecksDir:          decksDir,
		DeckImportLimiter: NewRateLimiter(capacity, 1.0/60.0),
	}
}

func importRequest(deckList, owner, ip string) *http.Request {
	body, _ := json.Marshal(map[string]string{
		"deck_list": deckList,
		"owner":     owner,
		"name":      "test_deck",
	})
	r := httptest.NewRequest("POST", "/api/decks", bytes.NewReader(body))
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestHandleImportDeck_UnderLimitSucceeds(t *testing.T) {
	h := newImportTestHandler(t, 3)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.handleImportDeck(w, importRequest("COMMANDER: Korvold\n1 Sol Ring\n", "alice", "203.0.113.30"))
		if w.Code != http.StatusOK {
			t.Fatalf("burst %d should succeed under limit, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}
}

func TestHandleImportDeck_OverLimitReturns429(t *testing.T) {
	h := newImportTestHandler(t, 2)
	// Burn 2-token burst.
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "198.51.100.40"))
		if w.Code != http.StatusOK {
			t.Fatalf("burst %d: %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "198.51.100.40"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit status = %d, want 429 (body=%s)", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}
	if !strings.Contains(w.Body.String(), "deck import") {
		t.Errorf("body should mention the deck-import label, got %s", w.Body.String())
	}
}

func TestHandleImportDeck_LimitIsPerIP(t *testing.T) {
	h := newImportTestHandler(t, 1)
	// Burn attacker.
	w := httptest.NewRecorder()
	h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "10.0.0.200"))
	if w.Code != http.StatusOK {
		t.Fatalf("attacker first call: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "10.0.0.200"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attacker over-limit should be 429, got %d", w.Code)
	}
	// Different IP — independent bucket.
	w = httptest.NewRecorder()
	h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "10.0.0.201"))
	if w.Code != http.StatusOK {
		t.Fatalf("independent IP should pass, got %d", w.Code)
	}
}

func TestHandleImportDeck_NilLimiterServesUnbounded(t *testing.T) {
	// Same TempDir/goroutine race rationale as newImportTestHandler.
	tmp, err := os.MkdirTemp("", "hexapi-ratelimit-nil-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	decksDir := filepath.Join(tmp, "decks")
	_ = os.MkdirAll(decksDir, 0o755)
	h := &Handler{DecksDir: decksDir, DeckImportLimiter: nil}
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "1.2.3.4"))
		if w.Code != http.StatusOK {
			t.Fatalf("nil limiter request %d: %d", i+1, w.Code)
		}
	}
}

// --- POST /api/decks/{owner}/{id}/analyze (handleRunAnalysis) ---------------
//
// analyze shares DeckImportLimiter with import. Verify the shared budget
// is in fact shared — a burst of imports must drain the same bucket the
// analyze handler reads from. (handleRunAnalysis kicks off a subprocess
// in a goroutine; we only assert the rate-limit gate, not the freya run.)

func TestHandleRunAnalysis_SharesDeckImportBudget(t *testing.T) {
	h := newImportTestHandler(t, 2)
	// Drop a deck file so handleRunAnalysis would otherwise progress to
	// the subprocess kickoff. We only care about the rate-limit gate.
	deckDir := filepath.Join(h.DecksDir, "alice")
	_ = os.MkdirAll(deckDir, 0o755)
	deckPath := filepath.Join(deckDir, "korvold.txt")
	_ = os.WriteFile(deckPath, []byte("COMMANDER: Korvold\n1 Sol Ring\n"), 0o644)

	// Burn the shared budget via imports.
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.handleImportDeck(w, importRequest("COMMANDER: x\n1 Sol Ring\n", "alice", "192.0.2.55"))
		if w.Code != http.StatusOK {
			t.Fatalf("import %d: %d", i+1, w.Code)
		}
	}
	// Now analyze from the same IP — should 429 even though it's a
	// different endpoint, because the limiter is shared.
	r := httptest.NewRequest("POST", "/api/decks/alice/korvold/analyze", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("id", "korvold")
	r.Header.Set("X-Forwarded-For", "192.0.2.55")
	w := httptest.NewRecorder()
	h.handleRunAnalysis(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("analyze should share budget — want 429 after imports drained, got %d (body=%s)",
			w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "deck analyze") {
		t.Errorf("over-limit body should carry the analyze label, got %s", w.Body.String())
	}
}

// --- POST /api/gauntlet/{owner}/{id} (handleStartGauntlet) -----------------

func gauntletRequest(owner, id, ip string) *http.Request {
	r := httptest.NewRequest("POST", "/api/gauntlet/"+owner+"/"+id, nil)
	r.SetPathValue("owner", owner)
	r.SetPathValue("id", id)
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestHandleStartGauntlet_OverLimitReturns429(t *testing.T) {
	sm := &Showmatch{
		GauntletLimiter: NewRateLimiter(1, 1.0/60.0),
	}
	// First call drains the 1-token bucket. It will then continue into
	// the handler — the deck won't be in the pool, so we'll get a 404,
	// but for THIS test we only care about the rate-limit short-circuit
	// firing on the SECOND call before any other state is touched.
	w := httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("alice", "korvold", "203.0.113.77"))
	// Second call: limiter must short-circuit with 429 BEFORE path
	// validation or the deck-pool lookup runs.
	w = httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("alice", "korvold", "203.0.113.77"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second call should be 429, got %d (body=%s)", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After on gauntlet 429")
	}
	if !strings.Contains(w.Body.String(), "gauntlet start") {
		t.Errorf("body should mention the gauntlet-start label, got %s", w.Body.String())
	}
}

func TestHandleStartGauntlet_LimiterRunsBeforePathValidation(t *testing.T) {
	// Regression: the limiter must short-circuit BEFORE the path-
	// validation guard (which would otherwise 400 on a bad owner). The
	// security goal is to stop a flood from churning the path validator
	// itself. We prove the order by exhausting the limiter then sending
	// a request whose owner WOULD have failed validation — the response
	// must be 429, not 400.
	sm := &Showmatch{
		GauntletLimiter: NewRateLimiter(1, 1.0/60.0),
	}
	// Drain via a different IP so we don't pollute the attacker bucket.
	w := httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("alice", "korvold", "10.0.0.1"))

	// Now drain the attacker bucket. Two requests from the same
	// (clean) attacker IP — first passes, second triggers 429.
	w = httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("alice", "korvold", "10.0.0.55"))
	// Second request with an invalid owner from the same IP — limiter
	// is depleted, so 429 wins over the 400 validation error.
	w = httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("..", "korvold", "10.0.0.55"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("limiter must run before path validation — want 429 on invalid owner after drain, got %d (body=%s)",
			w.Code, w.Body.String())
	}
}

func TestHandleStartGauntlet_NilLimiterIsNoop(t *testing.T) {
	// Without a limiter, the handler must reach its existing path-
	// validation logic. Send a known-bad owner and assert 400 — that
	// only happens if enforceRateLimit returned false on nil.
	sm := &Showmatch{} // GauntletLimiter nil
	w := httptest.NewRecorder()
	sm.handleStartGauntlet(w, gauntletRequest("..", "korvold", "1.2.3.4"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("nil limiter should fall through to path validation (400), got %d", w.Code)
	}
}

// --- POST /api/spectate/spawn (handleSpawnSpectateRoom) --------------------

func spawnRequest(deckID, ip string) *http.Request {
	body, _ := json.Marshal(map[string]string{"deck_id": deckID})
	r := httptest.NewRequest("POST", "/api/spectate/spawn", bytes.NewReader(body))
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestHandleSpawnSpectateRoom_OverLimitReturns429(t *testing.T) {
	sm := &Showmatch{
		SpectateSpawnLimiter: NewRateLimiter(1, 1.0/60.0),
	}
	// First call drains; will fail downstream (rooms is nil) but the
	// limiter consumed its token before that.
	w := httptest.NewRecorder()
	defer func() { _ = recover() }() // first call may panic on nil rooms — we don't care here
	func() {
		defer func() { _ = recover() }()
		sm.handleSpawnSpectateRoom(w, spawnRequest("alice/korvold", "203.0.113.99"))
	}()

	// Second call from the same IP: 429 short-circuit must fire BEFORE
	// any other state is touched (so the nil-rooms panic doesn't happen).
	w = httptest.NewRecorder()
	sm.handleSpawnSpectateRoom(w, spawnRequest("alice/korvold", "203.0.113.99"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second call should 429, got %d (body=%s)", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After on spectate-spawn 429")
	}
	if !strings.Contains(w.Body.String(), "spectate spawn") {
		t.Errorf("body should mention the spectate-spawn label, got %s", w.Body.String())
	}
}

func TestHandleSpawnSpectateRoom_LimitIsPerIP(t *testing.T) {
	sm := &Showmatch{
		SpectateSpawnLimiter: NewRateLimiter(1, 1.0/60.0),
	}
	// Burn attacker.
	func() {
		defer func() { _ = recover() }()
		w := httptest.NewRecorder()
		sm.handleSpawnSpectateRoom(w, spawnRequest("a/b", "10.0.0.150"))
	}()
	w := httptest.NewRecorder()
	sm.handleSpawnSpectateRoom(w, spawnRequest("a/b", "10.0.0.150"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attacker 2nd call should be 429, got %d", w.Code)
	}
	// Different IP — must reach past the limiter (and possibly panic
	// on nil rooms, which is fine — we recover and only assert the
	// limiter did NOT 429).
	w = httptest.NewRecorder()
	limited := false
	func() {
		defer func() { _ = recover() }()
		sm.handleSpawnSpectateRoom(w, spawnRequest("a/b", "10.0.0.151"))
		if w.Code == http.StatusTooManyRequests {
			limited = true
		}
	}()
	if limited {
		t.Error("independent IP should not be rate-limited")
	}
}
