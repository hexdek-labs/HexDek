package oracle

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// InboundRateLimiter is a per-IP token-bucket gate for the public
// /api/oracle/card/{name} endpoint. The oracle endpoint is
// unauthenticated and backed by a SQLite cache + opportunistic
// Scryfall fallback, so a single bad actor can otherwise spin a tight
// loop and exhaust either the DB connection pool or our outbound
// Scryfall budget (scryfallLimiter in limiter.go protects Scryfall but
// doesn't protect HexDek's own CPU/IO from the inbound burst).
//
// Same lazy-refill, namespaced-key shape as hexapi.RateLimiter; lives
// in the oracle package so this lower-level package doesn't take a
// dependency on hexapi.
type InboundRateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*ibBucket
	capacity        float64
	refillPerSecond float64
	now             func() time.Time // injectable for tests
}

type ibBucket struct {
	tokens     float64
	lastRefill time.Time
}

// NewInboundRateLimiter returns a token bucket with the given burst
// capacity and refill rate (tokens per second). Default for the oracle
// endpoint is capacity=60, refillPerSecond=1.0 — a 60-request burst
// then 60 req/min steady-state per IP.
func NewInboundRateLimiter(capacity, refillPerSecond float64) *InboundRateLimiter {
	return &InboundRateLimiter{
		buckets:         map[string]*ibBucket{},
		capacity:        capacity,
		refillPerSecond: refillPerSecond,
		now:             time.Now,
	}
}

// Allow returns (true, 0) if the request for `key` is within budget and
// consumes one token. Returns (false, retryAfter) when over-limit.
func (rl *InboundRateLimiter) Allow(key string) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := rl.now()
	b, ok := rl.buckets[key]
	if !ok {
		b = &ibBucket{tokens: rl.capacity - 1, lastRefill: now}
		rl.buckets[key] = b
		return true, 0
	}
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
	deficit := 1.0 - b.tokens
	if rl.refillPerSecond <= 0 {
		return false, time.Hour
	}
	secs := deficit / rl.refillPerSecond
	return false, time.Duration(secs * float64(time.Second))
}

// enforce writes the 429 + Retry-After response and returns true when
// over-budget. Returns false (writes nothing) when within budget or
// when the limiter is nil (back-compat no-op for older callers).
func (rl *InboundRateLimiter) enforce(w http.ResponseWriter, r *http.Request, label string) bool {
	if rl == nil {
		return false
	}
	ok, retryAfter := rl.Allow(clientIP(r))
	if ok {
		return false
	}
	secs := int(retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
	writeJSONErr(w, http.StatusTooManyRequests, label+" rate limit exceeded — too many requests")
	return true
}

// clientIP mirrors the hexapi extractor: honors the leftmost
// X-Forwarded-For entry (Caddy strips downstream proxy IPs), falls back
// to RemoteAddr's host, and uses "unknown" as a single safe-default
// bucket so a missing header can't grant unlimited budget.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
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
