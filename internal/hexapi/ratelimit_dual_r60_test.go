package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ratelimit_dual_r60_test.go — pins enforceRateLimitDual plus
// per-endpoint wiring for the three expensive surfaces that previously
// either had per-IP-only coverage (freya analyze, tournament run) or
// no coverage at all (spectator lineage). Two-axis defense matters:
// per-IP alone is bypassable by a single authenticated owner rotating
// source addresses (mobile + laptop + VPN); per-owner alone is
// bypassable by a botnet of unauthenticated requests with no owner
// header. Dual gating closes both holes.

// --- enforceRateLimitDual unit tests -------------------------------------

// helper: drive N dual-limiter requests through a recorder and return
// the per-request status codes. Each request can override the IP and
// owner header to model multi-source / multi-user load.
func driveDual(t *testing.T, perIP, perOwner *RateLimiter, n int, ip, owner string) []int {
	t.Helper()
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		r := httptest.NewRequest("POST", "/x", nil)
		if ip != "" {
			r.Header.Set("X-Forwarded-For", ip)
		}
		if owner != "" {
			r.Header.Set("X-HexDek-Owner", owner)
		}
		w := httptest.NewRecorder()
		if !enforceRateLimitDual(perIP, perOwner, w, r, "test") {
			w.WriteHeader(http.StatusOK)
		}
		codes[i] = w.Code
	}
	return codes
}

// Burst pattern: a 5-token burst over a single IP+owner pair flows
// past both gates in the first 5 requests and 429s on the 6th. The
// per-IP gate is consulted first; the X-RateLimit-By header tags
// which axis denied so operators can tell them apart in logs.
func TestEnforceRateLimitDual_BurstThenBlockOnPerIP(t *testing.T) {
	perIP := NewRateLimiter(5, 1.0/60.0)    // 5-burst, 1/min refill
	perOwner := NewRateLimiter(100, 10.0)   // intentionally generous so IP gate trips first

	codes := driveDual(t, perIP, perOwner, 5, "203.0.113.7", "alice")
	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("burst request %d code=%d, want 200", i+1, code)
		}
	}

	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.Header.Set("X-HexDek-Owner", "alice")
	w := httptest.NewRecorder()
	if !enforceRateLimitDual(perIP, perOwner, w, r, "test") {
		t.Fatal("6th request must over-limit")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit code=%d, want 429", w.Code)
	}
	if got := w.Header().Get("X-RateLimit-By"); got != "ip" {
		t.Errorf("X-RateLimit-By=%q, want ip (per-IP bucket trips first by design)", got)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("Retry-After missing from 429")
	}
}

// Burst pattern: a single owner rotating source IPs evades per-IP
// gating, but the per-owner gate still catches them. Verifies the
// SECOND axis works independently — without it, an attacker with N
// IPs can multiplex Nx burst.
func TestEnforceRateLimitDual_PerOwnerCatchesIPRotation(t *testing.T) {
	perIP := NewRateLimiter(5, 1.0/60.0)
	perOwner := NewRateLimiter(3, 1.0/60.0) // tight: 3 per minute per owner

	// Burn 3 requests from 3 different IPs — same owner.
	for i, ip := range []string{"203.0.113.1", "203.0.113.2", "203.0.113.3"} {
		r := httptest.NewRequest("POST", "/x", nil)
		r.Header.Set("X-Forwarded-For", ip)
		r.Header.Set("X-HexDek-Owner", "rotating-alice")
		w := httptest.NewRecorder()
		if enforceRateLimitDual(perIP, perOwner, w, r, "test") {
			t.Fatalf("request %d from %s should pass (owner has 3 budget, each IP has 5)", i+1, ip)
		}
	}
	// 4th: fresh IP that wouldn't otherwise be over-limit; the
	// per-owner bucket must reject.
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.4")
	r.Header.Set("X-HexDek-Owner", "rotating-alice")
	w := httptest.NewRecorder()
	if !enforceRateLimitDual(perIP, perOwner, w, r, "test") {
		t.Fatal("4th rotation must be caught by per-owner bucket")
	}
	if got := w.Header().Get("X-RateLimit-By"); got != "owner" {
		t.Errorf("X-RateLimit-By=%q, want owner (per-owner caught the IP-rotation evasion)", got)
	}
}

// Burst pattern: a per-IP burst from many anonymous (no owner header)
// callers all sharing one NAT IP gets caught even though no owner
// bucket exists. Verifies the FIRST axis works without the second.
func TestEnforceRateLimitDual_PerIPCatchesAnonymousNATAbuse(t *testing.T) {
	perIP := NewRateLimiter(3, 1.0/60.0)
	perOwner := NewRateLimiter(100, 10.0) // never tripped — no owner header sent

	// 3 anonymous bursts from same IP pass; 4th 429s.
	codes := driveDual(t, perIP, perOwner, 3, "10.0.0.99", "")
	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("anonymous burst %d code=%d, want 200", i+1, code)
		}
	}
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.99")
	w := httptest.NewRecorder()
	if !enforceRateLimitDual(perIP, perOwner, w, r, "test") {
		t.Fatal("4th anonymous burst must 429")
	}
	if got := w.Header().Get("X-RateLimit-By"); got != "ip" {
		t.Errorf("anonymous abuse should be tagged ip, got %q", got)
	}
}

// Sustained pattern: a request rate that respects the refill curve
// never trips either bucket, even past 2x the burst depth. Drives
// the injectable clock past the refill cycle to prove the lazy
// refill in RateLimiter.Allow actually replenishes during sustained
// throughput.
func TestEnforceRateLimitDual_SustainedAtRefillRateNeverBlocks(t *testing.T) {
	perIP := NewRateLimiter(3, 2.0) // 3-burst, 2/sec refill
	perOwner := NewRateLimiter(3, 2.0)
	now := time.Now()
	perIP.now = func() time.Time { return now }
	perOwner.now = func() time.Time { return now }

	// Drain both buckets to 0.
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest("POST", "/x", nil)
		r.Header.Set("X-Forwarded-For", "1.2.3.4")
		r.Header.Set("X-HexDek-Owner", "steady-eddie")
		w := httptest.NewRecorder()
		if enforceRateLimitDual(perIP, perOwner, w, r, "t") {
			t.Fatalf("drain %d should pass", i+1)
		}
	}

	// Sustained at 1 req every 500ms (= 2/sec, exactly the refill
	// rate). Step the clock between requests so the lazy refill
	// inside Allow sees real elapsed time. 8 sustained requests must
	// all pass — no accumulated denial.
	for i := 0; i < 8; i++ {
		now = now.Add(500 * time.Millisecond)
		r := httptest.NewRequest("POST", "/x", nil)
		r.Header.Set("X-Forwarded-For", "1.2.3.4")
		r.Header.Set("X-HexDek-Owner", "steady-eddie")
		w := httptest.NewRecorder()
		if enforceRateLimitDual(perIP, perOwner, w, r, "t") {
			t.Fatalf("sustained-at-refill request %d should pass; X-RateLimit-By=%q",
				i+1, w.Header().Get("X-RateLimit-By"))
		}
	}
}

// Sustained pattern: a request rate that EXCEEDS the refill curve
// trips on every request past the initial burst, with Retry-After
// reflecting the deficit. Confirms the gate doesn't accidentally
// over-replenish during fast bursts.
func TestEnforceRateLimitDual_SustainedAboveRefillBlocksTail(t *testing.T) {
	perIP := NewRateLimiter(3, 1.0) // 3-burst, 1/sec
	perOwner := NewRateLimiter(100, 100.0)
	now := time.Now()
	perIP.now = func() time.Time { return now }

	// 3-burst passes.
	pass, fail := 0, 0
	for i := 0; i < 10; i++ {
		now = now.Add(100 * time.Millisecond) // 10/sec → 10x faster than refill
		r := httptest.NewRequest("POST", "/x", nil)
		r.Header.Set("X-Forwarded-For", "9.9.9.9")
		w := httptest.NewRecorder()
		if enforceRateLimitDual(perIP, perOwner, w, r, "t") {
			fail++
		} else {
			pass++
		}
	}
	// First 3 burst tokens + ~1 accrued (1s × 1/sec across 10 × 100ms = 1.0)
	// ≤ 4 passes; the rest must 429.
	if pass > 4 {
		t.Errorf("too many passes: %d / 10 — refill over-credited", pass)
	}
	if fail < 6 {
		t.Errorf("too few denials: %d / 10 — burst exceeded but tail not throttled", fail)
	}
}

// Pins that the 429 body decodes as the unified ErrorResponse envelope
// (PR #778) — the rate-limit path must not silently revert to plain
// text under any keying mode.
func TestEnforceRateLimitDual_429UsesUnifiedEnvelope(t *testing.T) {
	perIP := NewRateLimiter(1, 1.0/60.0) // immediate over-limit on 2nd
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "5.5.5.5")
	w := httptest.NewRecorder()
	_ = enforceRateLimitDual(perIP, nil, w, r, "burst test")

	r = httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", "5.5.5.5")
	w = httptest.NewRecorder()
	if !enforceRateLimitDual(perIP, nil, w, r, "burst test") {
		t.Fatal("2nd request must over-limit")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("code=%d, want 429", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	var body ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, w.Body.String())
	}
	if body.Error.Code != "too_many_requests" {
		t.Errorf("error.code=%q, want too_many_requests", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "burst test") {
		t.Errorf("error.message=%q must mention endpoint label", body.Error.Message)
	}
	if body.Status != http.StatusTooManyRequests {
		t.Errorf("body.status=%d, want 429", body.Status)
	}
}

// Both limiters nil = no-op (backwards-compat for binaries that
// don't construct either axis yet).
func TestEnforceRateLimitDual_BothNilIsNoOp(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	w := httptest.NewRecorder()
	if enforceRateLimitDual(nil, nil, w, r, "x") {
		t.Fatal("nil/nil must not throttle")
	}
}

// --- Per-endpoint wiring tests -------------------------------------------

// freya analyze (POST /api/decks/{owner}/{id}/analyze): both axes
// active, per-owner bucket catches the IP-rotation evasion that
// per-IP alone would miss.
func TestHandleRunAnalysis_PerOwnerBucketCatchesIPRotation(t *testing.T) {
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	// findDeckFile expects {DecksDir}/{owner}/{id}.{txt,json}
	if err := os.MkdirAll(filepath.Join(decksDir, "alice"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(decksDir, "alice", "voltron.txt"), []byte("1 Sol Ring\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h := &Handler{
		DecksDir:                decksDir,
		DeckImportLimiter:       NewRateLimiter(100, 10.0),  // loose: per-IP won't trip
		DeckAnalyzeOwnerLimiter: NewRateLimiter(2, 1.0/60.0), // tight: 2 per minute per owner
	}

	// 2 analyze calls from 2 IPs, same owner → both pass (owner has 2).
	for i, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		r := httptest.NewRequest("POST", "/api/decks/alice/voltron/analyze", nil)
		r.SetPathValue("owner", "alice")
		r.SetPathValue("id", "voltron")
		r.Header.Set("X-Forwarded-For", ip)
		r.Header.Set("X-HexDek-Owner", "alice")
		w := httptest.NewRecorder()
		h.handleRunAnalysis(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("analyze %d from %s should pass: code=%d body=%q", i+1, ip, w.Code, w.Body.String())
		}
	}
	// 3rd call from fresh IP: per-IP bucket has tons of budget left,
	// but per-owner bucket is empty.
	r := httptest.NewRequest("POST", "/api/decks/alice/voltron/analyze", nil)
	r.SetPathValue("owner", "alice")
	r.SetPathValue("id", "voltron")
	r.Header.Set("X-Forwarded-For", "3.3.3.3")
	r.Header.Set("X-HexDek-Owner", "alice")
	w := httptest.NewRecorder()
	h.handleRunAnalysis(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd analyze from rotated IP must 429 — per-owner bucket should have caught it; code=%d", w.Code)
	}
	if got := w.Header().Get("X-RateLimit-By"); got != "owner" {
		t.Errorf("X-RateLimit-By=%q, want owner", got)
	}
}

// spectator lineage (GET /api/spectator/lineage/{id}): pre-r60 had no
// rate limit at all. Verifies the per-IP bucket now throttles burst.
// Per-owner not applicable here (lineage is unauthenticated by design).
func TestHandleSpectatorLineage_PerIPBurstNow429s(t *testing.T) {
	sm := &Showmatch{
		rooms:          NewRoomManager(),
		LineageLimiter: NewRateLimiter(2, 1.0/60.0), // tight: 2-burst
	}

	// 2 well-formed-but-unknown lineage queries pass (consume the burst).
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("GET", "/api/spectator/lineage/h0OGVW20042", nil)
		r.SetPathValue("instance_id", "h0OGVW20042")
		r.Header.Set("X-Forwarded-For", "203.0.113.50")
		w := httptest.NewRecorder()
		sm.handleSpectatorLineage(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("burst %d should pass to 404, got %d", i+1, w.Code)
		}
	}
	// 3rd burst from same IP: 429.
	r := httptest.NewRequest("GET", "/api/spectator/lineage/h0OGVW20042", nil)
	r.SetPathValue("instance_id", "h0OGVW20042")
	r.Header.Set("X-Forwarded-For", "203.0.113.50")
	w := httptest.NewRecorder()
	sm.handleSpectatorLineage(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd lineage query must 429, got %d", w.Code)
	}
	// Independent IP unaffected — verifies per-IP keying isolates.
	r = httptest.NewRequest("GET", "/api/spectator/lineage/h0OGVW20042", nil)
	r.SetPathValue("instance_id", "h0OGVW20042")
	r.Header.Set("X-Forwarded-For", "203.0.113.99")
	w = httptest.NewRecorder()
	sm.handleSpectatorLineage(w, r)
	if w.Code == http.StatusTooManyRequests {
		t.Errorf("independent IP must NOT inherit attacker's throttle; got %d", w.Code)
	}
}

// Spectator lineage: format validation runs BEFORE rate-limit so a
// flood of garbage IDs from one IP doesn't burn its budget on
// requests that never touch the rooms scan. Defense in depth — the
// scan is the cost we care about; bad-format rejects are cheap and
// shouldn't count.
func TestHandleSpectatorLineage_FormatRejectIsFree(t *testing.T) {
	sm := &Showmatch{
		rooms:          NewRoomManager(),
		LineageLimiter: NewRateLimiter(2, 1.0/60.0),
	}
	// Spam 50 format-invalid IDs — none consume budget.
	for i := 0; i < 50; i++ {
		r := httptest.NewRequest("GET", "/api/spectator/lineage/garbage", nil)
		r.SetPathValue("instance_id", "garbage")
		r.Header.Set("X-Forwarded-For", "5.5.5.5")
		w := httptest.NewRecorder()
		sm.handleSpectatorLineage(w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid id call %d code=%d, want 400", i+1, w.Code)
		}
	}
	// Now 2 valid-format requests must still pass — the bad-format
	// flood did NOT consume budget.
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("GET", "/api/spectator/lineage/h0OGVW20042", nil)
		r.SetPathValue("instance_id", "h0OGVW20042")
		r.Header.Set("X-Forwarded-For", "5.5.5.5")
		w := httptest.NewRecorder()
		sm.handleSpectatorLineage(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("post-garbage valid request %d code=%d, want 404 (passes limiter)", i+1, w.Code)
		}
	}
}

// Nil-safety: handleSpectatorLineage with neither limiter configured
// behaves exactly as it did pre-r60 (no throttling, just the 404 path
// after the rooms scan). Pin so dropping a limiter in a future
// constructor refactor doesn't accidentally regress to "endpoint
// rejects everything because limiter is required."
func TestHandleSpectatorLineage_NilLimitersAreBackwardsCompatible(t *testing.T) {
	sm := &Showmatch{rooms: NewRoomManager()} // no limiters

	for i := 0; i < 20; i++ {
		r := httptest.NewRequest("GET", "/api/spectator/lineage/h0OGVW20042", nil)
		r.SetPathValue("instance_id", "h0OGVW20042")
		r.Header.Set("X-Forwarded-For", "1.1.1.1")
		w := httptest.NewRecorder()
		sm.handleSpectatorLineage(w, r)
		if w.Code != http.StatusNotFound {
			t.Fatalf("unlimited request %d code=%d, want 404", i+1, w.Code)
		}
	}
}
