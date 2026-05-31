package ws

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// throttle.go — WebSocket throttling primitives.
//
// Three protection layers, each independently nil-safe so a binary
// that hasn't wired any of them keeps the pre-r60 behavior:
//
//  1. Per-IP upgrade gate (upgradeLimiter)
//     Pre-r60 the /ws/party/{id} upgrade path did unauthenticated DB
//     work (auth.ValidateSession + ListPartyMembers) on every connect
//     before deciding to upgrade. A reconnect-storm from one IP burned
//     those lookups indefinitely. Token bucket on the upgrade path
//     keyed by client IP throttles the storm before any DB hit.
//
//  2. Per-connection inbound message gate (connBucket)
//     Pre-r60 every inbound message on a live connection went straight
//     to dispatch — most game.* actions trigger a snapshot broadcast
//     that fans out to all party members. A malicious client sending
//     `game.advance_phase` in a tight loop amplifies one inbound
//     message into N×M (N members × M snapshot fetches per member).
//     Per-connection bucket caps the amplification at the source.
//
//  3. Unified error envelope on upgrade-failure paths (writeWSError)
//     Pre-r60 the 3 pre-upgrade error sites used plain-text http.Error,
//     breaking the {error:{code,message}, request_id, status} envelope
//     contract established for hexapi by PR #805 / #843. Mirroring the
//     shape here so WebSocket clients parsing error.code get the same
//     guarantee as HTTP clients. Cannot import internal/hexapi
//     (ws + hexapi are sibling packages — no cycle today, but adding
//     the import would create one if hexapi ever pulls in ws), so
//     the envelope shape is duplicated.

// tokenBucket is a tiny per-key token-bucket. Lighter than the hexapi
// equivalent (no shared map state, no label, no nil-safe wrapper);
// constructed per-connection or per-IP-key and lives only as long as
// its caller needs it.
type tokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	now        func() time.Time // injectable for tests
}

// newTokenBucket starts at full capacity. capacity ≤ 0 returns nil
// (caller treats nil as "no limit") — keeps the call sites readable.
func newTokenBucket(capacity int, refillPerSec float64) *tokenBucket {
	if capacity <= 0 {
		return nil
	}
	now := time.Now
	return &tokenBucket{
		tokens:     float64(capacity),
		capacity:   float64(capacity),
		refillRate: refillPerSec,
		lastRefill: now(),
		now:        now,
	}
}

// allow returns (true, 0) if the request is within budget and consumes
// one token. Returns (false, retryAfter) when over-limit, where
// retryAfter is the duration until the next whole token becomes
// available. A nil bucket always allows (the "no limit" branch).
func (b *tokenBucket) allow() (bool, time.Duration) {
	if b == nil {
		return true, 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.refillRate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.lastRefill = now
	}
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, 0
	}
	if b.refillRate <= 0 {
		return false, time.Hour // misconfigured (capacity>0 but zero refill)
	}
	deficit := 1.0 - b.tokens
	secs := deficit / b.refillRate
	return false, time.Duration(secs * float64(time.Second))
}

// upgradeLimiter is the per-IP throttle used on the WebSocket upgrade
// path. Buckets live in a single map keyed by IP. Bucket entries are
// retained for the life of the process (memory bounded by unique
// client IPs); idle entries refill to full and don't accumulate.
type upgradeLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*tokenBucket
	capacity int
	refill   float64
}

func newUpgradeLimiter(capacity int, refillPerSec float64) *upgradeLimiter {
	if capacity <= 0 {
		return nil
	}
	return &upgradeLimiter{
		buckets:  map[string]*tokenBucket{},
		capacity: capacity,
		refill:   refillPerSec,
	}
}

// allow returns (true, 0) if this IP's upgrade is within budget.
// Nil limiter always allows.
func (u *upgradeLimiter) allow(ip string) (bool, time.Duration) {
	if u == nil {
		return true, 0
	}
	u.mu.Lock()
	b, ok := u.buckets[ip]
	if !ok {
		b = newTokenBucket(u.capacity, u.refill)
		u.buckets[ip] = b
	}
	u.mu.Unlock()
	return b.allow()
}

// errorBody mirrors hexapi.ErrorBody. Duplicated rather than imported
// to keep internal/ws standalone — see throttle.go docstring.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// errorResponse mirrors hexapi.ErrorResponse — see errorBody.
type errorResponse struct {
	Error  errorBody `json:"error"`
	Status int       `json:"status"`
}

// writeWSError writes a JSON ErrorResponse-shaped 4xx/5xx body. Used on
// every pre-upgrade error path (bad party id, unauthorized, not a
// member, rate-limited) so WebSocket clients reading the rejection
// body get the same nested-envelope shape as every other hexapi
// caller per PR #805.
func writeWSError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	body := errorResponse{
		Error:  errorBody{Code: code, Message: message},
		Status: status,
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("ws writeWSError encode: %v", err)
	}
}

// clientIP mirrors hexapi.clientIP — leftmost X-Forwarded-For trusted
// (Caddy on MISTY strips downstream proxy IPs); falls back to
// RemoteAddr's host. Returns "unknown" so a missing-everything caller
// still buckets (one shared "unknown" bucket — the safe default).
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
