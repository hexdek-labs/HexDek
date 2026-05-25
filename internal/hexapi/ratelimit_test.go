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
	"time"
)

func TestRateLimiter_UnderLimitAllowsBurst(t *testing.T) {
	rl := NewRateLimiter(5, 1.0/60.0) // 5-burst, 1/min refill
	for i := 0; i < 5; i++ {
		ok, wait := rl.Allow("1.2.3.4")
		if !ok {
			t.Fatalf("request %d should be allowed (5-token burst), got over-limit with wait=%v", i+1, wait)
		}
	}
}

func TestRateLimiter_OverLimitReturnsRetryAfter(t *testing.T) {
	rl := NewRateLimiter(3, 1.0/60.0)
	for i := 0; i < 3; i++ {
		if ok, _ := rl.Allow("attacker"); !ok {
			t.Fatalf("burst request %d should be allowed", i+1)
		}
	}
	// 4th request: over-limit.
	ok, retry := rl.Allow("attacker")
	if ok {
		t.Fatal("4th request must be over-limit")
	}
	if retry <= 0 {
		t.Fatalf("retry-after must be positive, got %v", retry)
	}
	// With refill = 1/min, retry should be ~1 minute (60s).
	if retry < 30*time.Second || retry > 90*time.Second {
		t.Errorf("retry=%v outside expected ~60s window", retry)
	}
}

func TestRateLimiter_DifferentKeysIndependentBuckets(t *testing.T) {
	rl := NewRateLimiter(2, 1.0/60.0)
	for i := 0; i < 2; i++ {
		if ok, _ := rl.Allow("alice"); !ok {
			t.Fatalf("alice request %d should be allowed", i+1)
		}
	}
	if ok, _ := rl.Allow("alice"); ok {
		t.Fatal("alice 3rd request should be over-limit")
	}
	// Bob's bucket is independent — full capacity.
	for i := 0; i < 2; i++ {
		if ok, _ := rl.Allow("bob"); !ok {
			t.Fatalf("bob request %d should be allowed (his bucket is independent)", i+1)
		}
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(2, 1.0) // 2-burst, 1 per second
	// Inject a controllable clock.
	now := time.Now()
	rl.now = func() time.Time { return now }

	// Drain.
	for i := 0; i < 2; i++ {
		if ok, _ := rl.Allow("k"); !ok {
			t.Fatalf("drain %d should pass", i+1)
		}
	}
	if ok, _ := rl.Allow("k"); ok {
		t.Fatal("3rd should be blocked at t=0")
	}

	// Advance 1.5 seconds → 1.5 tokens accrued → first request passes,
	// second blocks (only 0.5 tokens left).
	now = now.Add(1500 * time.Millisecond)
	if ok, _ := rl.Allow("k"); !ok {
		t.Fatal("after 1.5s a token must be available")
	}
	if ok, _ := rl.Allow("k"); ok {
		t.Fatal("only one token accrued; second post-refill request must block")
	}

	// Another second → 1 more token, request passes.
	now = now.Add(1 * time.Second)
	if ok, _ := rl.Allow("k"); !ok {
		t.Fatal("after additional 1s another token must be available")
	}
}

func TestRateLimiter_DoesNotExceedCapacityOnLongIdle(t *testing.T) {
	rl := NewRateLimiter(3, 10.0) // 3-burst, 10/sec
	now := time.Now()
	rl.now = func() time.Time { return now }
	// Prime the bucket so it's tracked.
	if ok, _ := rl.Allow("idle"); !ok {
		t.Fatal("first request should pass")
	}
	// Idle for an hour — refill MUST cap at capacity, not balloon to
	// 10*3600 tokens.
	now = now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if ok, _ := rl.Allow("idle"); !ok {
			t.Fatalf("post-idle request %d should pass (capacity = 3)", i+1)
		}
	}
	if ok, _ := rl.Allow("idle"); ok {
		t.Fatal("4th post-idle request must block — capacity must NOT have ballooned past 3")
	}
}

func TestClientIP_PrefersXForwardedFor(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.RemoteAddr = "10.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("want X-Forwarded-For ip, got %q", got)
	}
}

func TestClientIP_LeftmostXForwardedForEntry(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.Header.Set("X-Forwarded-For", " 203.0.113.7 , 198.51.100.2 , 10.0.0.5 ")
	if got := clientIP(r); got != "203.0.113.7" {
		t.Errorf("want leftmost entry, got %q", got)
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.RemoteAddr = "192.0.2.1:443"
	if got := clientIP(r); got != "192.0.2.1" {
		t.Errorf("want RemoteAddr host, got %q", got)
	}
}

func TestClientIP_HandlesEmptyEverything(t *testing.T) {
	r := httptest.NewRequest("POST", "/x", nil)
	r.RemoteAddr = ""
	if got := clientIP(r); got != "unknown" {
		t.Errorf("empty RemoteAddr + no header should fall through to %q, got %q", "unknown", got)
	}
}

// --- end-to-end handler tests ---

// newFeedbackTestHandler builds a Handler with a temp feedback dir and
// a tight rate-limiter (3 burst, 1/min refill) so over-limit triggers
// quickly. Returns the handler + the dir so tests can poke at the
// produced files.
func newFeedbackTestHandler(t *testing.T) (*Handler, string) {
	t.Helper()
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	if err := os.MkdirAll(decksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &Handler{
		DecksDir:        decksDir,
		FeedbackLimiter: NewRateLimiter(3, 1.0/60.0),
	}
	return h, decksDir
}

func feedbackRequest(body map[string]string, ip string) *http.Request {
	buf, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", "/api/feedback", bytes.NewReader(buf))
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func TestHandleFeedback_UnderLimitWritesFile(t *testing.T) {
	h, dir := newFeedbackTestHandler(t)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.handleFeedback(w, feedbackRequest(map[string]string{
			"type":    "bug",
			"symptom": "thing broke",
		}, "203.0.113.7"))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d should succeed under limit, got %d: %s", i+1, w.Code, w.Body.String())
		}
		// Sleep 2ms between requests so the millisecond-keyed filename
		// scheme in handleFeedback doesn't collide. The filename
		// collision is a pre-existing condition unrelated to this
		// rate-limit fix; the assertion below only proves the
		// under-limit path writes ≥1 file rather than every request.
		time.Sleep(2 * time.Millisecond)
	}
	feedbackDir := filepath.Join(dir, "..", "feedback")
	entries, err := os.ReadDir(feedbackDir)
	if err != nil {
		t.Fatalf("feedback dir: %v", err)
	}
	if len(entries) < 1 {
		t.Errorf("expected at least 1 feedback file under limit, got %d", len(entries))
	}
}

func TestHandleFeedback_OverLimitReturns429WithRetryAfter(t *testing.T) {
	h, _ := newFeedbackTestHandler(t)
	// Burn the 3-token burst.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "x"}, "198.51.100.9"))
		if w.Code != http.StatusOK {
			t.Fatalf("burst %d should succeed, got %d", i+1, w.Code)
		}
	}
	// 4th request from the SAME IP: over-limit.
	w := httptest.NewRecorder()
	h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "y"}, "198.51.100.9"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit response code = %d, want 429", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("over-limit response missing Retry-After header")
	}
	if !strings.Contains(w.Body.String(), "rate limit") &&
		!strings.Contains(w.Body.String(), "too many") {
		t.Errorf("over-limit body should mention rate limit, got %s", w.Body.String())
	}
}

func TestHandleFeedback_LimitIsPerIP(t *testing.T) {
	h, _ := newFeedbackTestHandler(t)
	// Burn attacker's budget.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "x"}, "10.0.0.99"))
		if w.Code != http.StatusOK {
			t.Fatalf("attacker burst %d: %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "x"}, "10.0.0.99"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attacker should be 429, got %d", w.Code)
	}
	// Legitimate user from a different IP must still be served.
	w = httptest.NewRecorder()
	h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "legit"}, "10.0.0.100"))
	if w.Code != http.StatusOK {
		t.Fatalf("legitimate user from independent IP should pass, got %d: %s",
			w.Code, w.Body.String())
	}
}

func TestHandleFeedback_NilLimiterIsBackwardsCompatible(t *testing.T) {
	tmp := t.TempDir()
	decksDir := filepath.Join(tmp, "decks")
	_ = os.MkdirAll(decksDir, 0o755)
	h := &Handler{DecksDir: decksDir, FeedbackLimiter: nil}

	// Without a limiter configured, the endpoint must continue to serve
	// requests (existing server binaries that don't yet construct a
	// limiter must keep working).
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		h.handleFeedback(w, feedbackRequest(map[string]string{"symptom": "x"}, "1.2.3.4"))
		if w.Code != http.StatusOK {
			t.Fatalf("nil limiter request %d: %d", i+1, w.Code)
		}
	}
}
