package hexapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimiter is a simple per-key token-bucket. Used to protect cheap
// unauthenticated POST endpoints from low-effort abuse (anonymous
// feedback spam, telemetry blasts, etc.). For higher-stakes endpoints
// we already have purpose-built protections — `handleCloneDeck` uses a
// DB-backed clone-log counter; this primitive is for in-memory, fast-
// path checks where the cost of a single request is bounded but the
// accumulated cost across an attacker's burst would burn disk / CPU.
//
// Per-key state: tokens (current capacity, float for sub-token refill)
// + lastRefill (last refill clock-read). Refill is computed lazily on
// each Allow call, so an idle key consumes O(1) work between requests.
//
// Memory growth is bounded only by unique keys; cleanup happens
// opportunistically inside Allow when a bucket has been idle long
// enough to have refilled to full — those entries are deletable
// without losing any state. Heavy load with churned keys may still
// grow the map; deploy with a parent process supervisor + restart on
// memory ceiling if it ever becomes a concern (MVP scale: a few
// thousand unique IPs per day is fine).
type RateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*rlBucket
	capacity        float64
	refillPerSecond float64
	now             func() time.Time // injectable for tests
}

type rlBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NewRateLimiter returns a token bucket with the given burst capacity
// and refill rate (tokens per second). Sensible defaults for the
// feedback endpoint: capacity=5, refillPerSecond=1.0/60 (5-request
// burst, then 1 request per minute).
func NewRateLimiter(capacity, refillPerSecond float64) *RateLimiter {
	return &RateLimiter{
		buckets:         map[string]*rlBucket{},
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		now:             time.Now,
	}
}

// Allow returns (true, 0) if the request for `key` is within budget
// and consumes one token. Returns (false, retryAfter) when over-limit;
// retryAfter is how long until the next whole token becomes available.
func (rl *RateLimiter) Allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	b, ok := rl.buckets[key]
	if !ok {
		// Fresh bucket starts at full capacity. Charge one token and
		// record the consumption immediately so the first request
		// counts toward the budget.
		b = &rlBucket{tokens: rl.capacity - 1, lastRefill: now}
		rl.buckets[key] = b
		return true, 0
	}
	// Lazy refill based on elapsed wall time.
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * rl.refillPerSecond
		if b.tokens > rl.capacity {
			b.tokens = rl.capacity
		}
		b.lastRefill = now
	}
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, 0
	}
	// Over-budget: tell the caller how long to wait for a whole token.
	deficit := 1.0 - b.tokens
	if rl.refillPerSecond <= 0 {
		return false, time.Hour // misconfigured: don't divide by zero
	}
	secs := deficit / rl.refillPerSecond
	return false, time.Duration(secs * float64(time.Second))
}

// clientIP extracts the originating client IP from an http.Request.
// Honors X-Forwarded-For when the server is behind a reverse proxy
// (Caddy on MISTY in our deployment); falls back to RemoteAddr's host
// when the header is absent. Returns "unknown" if both are empty —
// a single "unknown" bucket then aggregates anonymous traffic, which
// is the safe default (one bad header doesn't grant unlimited budget).
//
// We trust only the LEFTMOST X-Forwarded-For entry (the original
// client). Caddy strips downstream proxy IPs; if we ever sit behind
// multiple proxies we'd need to walk the chain back from the right.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Header can be a comma-separated chain.
		if comma := strings.IndexByte(xff, ','); comma >= 0 {
			xff = xff[:comma]
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
