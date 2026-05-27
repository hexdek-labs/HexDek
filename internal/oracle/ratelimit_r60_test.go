package oracle

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestInboundRateLimiter_AllowsWithinBurst confirms the first N requests
// (where N == capacity) all return ok. Pins the burst contract: a real
// client paging through their decklist needs to fire ~60 lookups back
// to back without artificial spacing.
func TestInboundRateLimiter_AllowsWithinBurst(t *testing.T) {
	rl := NewInboundRateLimiter(60, 1.0)
	for i := 0; i < 60; i++ {
		ok, retry := rl.Allow("1.2.3.4")
		if !ok {
			t.Fatalf("request %d rejected within burst (retry=%s)", i, retry)
		}
	}
}

// TestInboundRateLimiter_RejectsOverBurst pins the 61st request as
// over-budget with a positive retry-after.
func TestInboundRateLimiter_RejectsOverBurst(t *testing.T) {
	rl := NewInboundRateLimiter(60, 1.0)
	// Pin time so refill doesn't sneak in a token mid-test.
	frozen := time.Unix(1_700_000_000, 0)
	rl.now = func() time.Time { return frozen }
	for i := 0; i < 60; i++ {
		if ok, _ := rl.Allow("1.2.3.4"); !ok {
			t.Fatalf("request %d unexpectedly rejected within burst", i)
		}
	}
	ok, retry := rl.Allow("1.2.3.4")
	if ok {
		t.Fatal("request 61 should be over-budget")
	}
	if retry <= 0 {
		t.Fatalf("retry-after must be positive, got %s", retry)
	}
	// 1 req/sec refill → next whole token in ≤ 1s.
	if retry > time.Second+50*time.Millisecond {
		t.Fatalf("retry-after for 1 req/sec refill should be ~1s, got %s", retry)
	}
}

// TestInboundRateLimiter_PerKeyIsolation confirms two distinct IPs
// don't share a bucket — an attacker exhausting their own budget can't
// drag a legitimate user's budget down with them.
func TestInboundRateLimiter_PerKeyIsolation(t *testing.T) {
	rl := NewInboundRateLimiter(2, 1.0)
	for i := 0; i < 2; i++ {
		if ok, _ := rl.Allow("attacker"); !ok {
			t.Fatalf("attacker request %d unexpectedly rejected", i)
		}
	}
	if ok, _ := rl.Allow("attacker"); ok {
		t.Fatal("attacker should be over-budget after 2 requests")
	}
	// Victim should still have full budget.
	if ok, _ := rl.Allow("victim"); !ok {
		t.Fatal("victim must not be affected by attacker's bucket")
	}
}

// TestInboundRateLimiter_RefillRestoresBudget confirms tokens come back
// at the configured rate. Uses injected clock so the test is fast +
// deterministic — no real sleeps.
func TestInboundRateLimiter_RefillRestoresBudget(t *testing.T) {
	rl := NewInboundRateLimiter(2, 1.0) // 1 token/sec
	clock := time.Unix(1_700_000_000, 0)
	rl.now = func() time.Time { return clock }

	rl.Allow("x")
	rl.Allow("x")
	if ok, _ := rl.Allow("x"); ok {
		t.Fatal("3rd request must be over-budget at t=0")
	}
	clock = clock.Add(1500 * time.Millisecond) // +1.5 tokens
	if ok, _ := rl.Allow("x"); !ok {
		t.Fatal("4th request should be allowed after 1.5s of refill")
	}
	// Only 0.5 tokens left — next request must fail.
	if ok, _ := rl.Allow("x"); ok {
		t.Fatal("5th request should still be over-budget (0.5 tokens left)")
	}
}

// TestEnforce_Returns429AndRetryAfter pins the wire-format contract:
// over-budget responses must be HTTP 429 with a Retry-After header
// that's a positive integer second count (RFC 6585 / RFC 7231 §7.1.3).
func TestEnforce_Returns429AndRetryAfter(t *testing.T) {
	rl := NewInboundRateLimiter(1, 1.0)
	frozen := time.Unix(1_700_000_000, 0)
	rl.now = func() time.Time { return frozen }

	// Consume the single token.
	req1 := httptest.NewRequest("GET", "/api/oracle/card/island", nil)
	req1.RemoteAddr = "9.9.9.9:1234"
	w1 := httptest.NewRecorder()
	if rl.enforce(w1, req1, "oracle lookup") {
		t.Fatal("first request must not be rate-limited")
	}

	// Second request from same IP must 429.
	req2 := httptest.NewRequest("GET", "/api/oracle/card/island", nil)
	req2.RemoteAddr = "9.9.9.9:1234"
	w2 := httptest.NewRecorder()
	if !rl.enforce(w2, req2, "oracle lookup") {
		t.Fatal("second request should be rate-limited")
	}
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
	ra := w2.Header().Get("Retry-After")
	if ra == "" {
		t.Fatal("Retry-After header missing")
	}
	secs, err := strconv.Atoi(ra)
	if err != nil {
		t.Fatalf("Retry-After must be integer seconds, got %q (%v)", ra, err)
	}
	if secs < 1 {
		t.Fatalf("Retry-After must be >= 1 second, got %d", secs)
	}
	body := w2.Body.String()
	if !strings.Contains(body, "oracle lookup") || !strings.Contains(body, "rate limit") {
		t.Fatalf("body should mention label + rate limit, got %q", body)
	}
}

// TestEnforce_NilLimiterIsNoOp confirms a nil limiter passes every
// request through. Lets callers compose Handler without a limiter
// (e.g. in low-trust tests) without panicking.
func TestEnforce_NilLimiterIsNoOp(t *testing.T) {
	var rl *InboundRateLimiter
	req := httptest.NewRequest("GET", "/api/oracle/card/x", nil)
	w := httptest.NewRecorder()
	if rl.enforce(w, req, "oracle lookup") {
		t.Fatal("nil limiter should never reject")
	}
	if w.Code != 200 && w.Code != 0 {
		t.Fatalf("nil limiter should not write a response, got code %d", w.Code)
	}
}

// TestClientIP_HonorsXForwardedFor confirms the proxy-aware extractor
// picks the leftmost X-Forwarded-For entry. We sit behind Caddy on
// MISTY, so RemoteAddr is always the proxy — without this, every
// request shares one bucket and the per-IP guarantee is meaningless.
func TestClientIP_HonorsXForwardedFor(t *testing.T) {
	tests := []struct {
		name  string
		xff   string
		raddr string
		want  string
	}{
		{"single XFF", "203.0.113.1", "127.0.0.1:9999", "203.0.113.1"},
		{"chained XFF picks leftmost", "203.0.113.1, 10.0.0.5, 10.0.0.1", "127.0.0.1:9999", "203.0.113.1"},
		{"XFF with whitespace", "  203.0.113.7  ", "127.0.0.1:9999", "203.0.113.7"},
		{"falls back to RemoteAddr host", "", "198.51.100.42:55432", "198.51.100.42"},
		{"empty everything returns unknown", "", "", "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/x", nil)
			req.RemoteAddr = tc.raddr
			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}
			got := clientIP(req)
			if got != tc.want {
				t.Fatalf("clientIP(xff=%q raddr=%q) = %q, want %q", tc.xff, tc.raddr, got, tc.want)
			}
		})
	}
}

// TestHandlerRegister_InstallsDefaultLimiter pins the deployment-safety
// contract: a Handler constructed without an explicit Limiter must NOT
// be left wide-open after Register. Defends against future refactors
// that might "simplify" the constructor and drop the default.
func TestHandlerRegister_InstallsDefaultLimiter(t *testing.T) {
	h := &Handler{} // no Limiter
	mux := http.NewServeMux()
	h.Register(mux)
	if h.Limiter == nil {
		t.Fatal("Register must install a default limiter when none was provided")
	}
	if h.Limiter.capacity != 60 {
		t.Fatalf("default capacity should be 60, got %v", h.Limiter.capacity)
	}
	if h.Limiter.refillPerSecond != 1.0 {
		t.Fatalf("default refill should be 1.0/sec (≈ 60 req/min), got %v", h.Limiter.refillPerSecond)
	}
}

// TestHandlerRegister_PreservesCustomLimiter confirms an explicit
// Limiter is NOT overwritten — callers (cmd/hexdek-server, tests) need
// to be able to tune burst/refill for their own workloads.
func TestHandlerRegister_PreservesCustomLimiter(t *testing.T) {
	custom := NewInboundRateLimiter(5, 0.5)
	h := &Handler{Limiter: custom}
	h.Register(http.NewServeMux())
	if h.Limiter != custom {
		t.Fatal("Register must not replace a caller-provided limiter")
	}
}

// TestLookupCard_RateLimitedReturns429 end-to-end check that the
// lookupCard HTTP path actually invokes the limiter — pins the wiring,
// not just the primitive. Uses a stubbed Scryfall + tmp SQLite DB so
// the first request returns a normal 200/404 (whichever), and the
// second from the same IP is the 429 we care about.
func TestLookupCard_RateLimitedReturns429(t *testing.T) {
	stub := newScryfallStub(t, time.Millisecond, nil)
	defer stub.close()
	defer SetScryfallBaseForTest(stub.server.URL)()
	defer SetScryfallLimitIntervalForTest(time.Millisecond)()

	h := &Handler{
		DB:      newOracleTestDB(t),
		Limiter: NewInboundRateLimiter(1, 1.0),
	}
	frozen := time.Unix(1_700_000_000, 0)
	h.Limiter.now = func() time.Time { return frozen }
	mux := http.NewServeMux()
	h.Register(mux)

	// First request consumes the token. Outcome (200/404/500) is
	// irrelevant — what matters is that the limiter let it through.
	req1 := httptest.NewRequest("GET", "/api/oracle/card/island", nil)
	req1.RemoteAddr = "7.7.7.7:1"
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, req1)
	if w1.Code == http.StatusTooManyRequests {
		t.Fatal("first request should not be rate-limited")
	}

	// Second request from the same IP must be 429 BEFORE any DB call —
	// the limiter check sits at the top of lookupCard.
	req2 := httptest.NewRequest("GET", "/api/oracle/card/island", nil)
	req2.RemoteAddr = "7.7.7.7:1"
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d (body=%q)", w2.Code, w2.Body.String())
	}
	if w2.Header().Get("Retry-After") == "" {
		t.Fatal("429 response must carry Retry-After header")
	}
}
