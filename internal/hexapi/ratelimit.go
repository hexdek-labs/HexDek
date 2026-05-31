package hexapi

import (
	"fmt"
	"net"
	"net/http"
	"sort"
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
	denials         uint64           // process-lifetime over-limit count, for /api/admin/ratelimit
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
	// Bump the lifetime denial counter so /api/admin/ratelimit can
	// surface "how loud has this limiter been" without sampling.
	rl.denials++
	deficit := 1.0 - b.tokens
	if rl.refillPerSecond <= 0 {
		return false, time.Hour // misconfigured: don't divide by zero
	}
	secs := deficit / rl.refillPerSecond
	return false, time.Duration(secs * float64(time.Second))
}

// Snapshot is the read-only view of a single RateLimiter exposed by
// /api/admin/ratelimit. Captured under the limiter's mutex so the
// returned state is internally consistent (capacity / refill_per_sec
// pair with buckets observed at the same instant).
type Snapshot struct {
	Capacity        float64          `json:"capacity"`
	RefillPerSecond float64          `json:"refill_per_sec"`
	Denials         uint64           `json:"denials"`
	BucketCount     int              `json:"bucket_count"`
	Buckets         []BucketSnapshot `json:"buckets"`
}

// BucketSnapshot is one row in Snapshot.Buckets. Key is whatever the
// caller bucketed by — "ip:1.2.3.4" / "owner:alice" / bare IP — so
// operators can see which user / IP is hot without grepping logs.
type BucketSnapshot struct {
	Key             string  `json:"key"`
	Tokens          float64 `json:"tokens"`
	LastRefillUnix  int64   `json:"last_refill_unix"`
}

// SnapshotState returns the current bucket states + lifetime denial
// count. Used by /api/admin/ratelimit. Caller controls cardinality
// via maxBuckets — pass <= 0 for unbounded. When the limit is hit
// the returned slice is the lexicographically-first maxBuckets keys
// (so output is deterministic across calls).
func (rl *RateLimiter) SnapshotState(maxBuckets int) Snapshot {
	if rl == nil {
		return Snapshot{}
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()
	keys := make([]string, 0, len(rl.buckets))
	for k := range rl.buckets {
		keys = append(keys, k)
	}
	// Sort so output is stable across calls and so the trim window
	// is deterministic (alphabetical-first maxBuckets keys).
	sort.Strings(keys)
	trim := len(keys)
	if maxBuckets > 0 && trim > maxBuckets {
		trim = maxBuckets
	}
	out := make([]BucketSnapshot, 0, trim)
	for i := 0; i < trim; i++ {
		b := rl.buckets[keys[i]]
		out = append(out, BucketSnapshot{
			Key:            keys[i],
			Tokens:         b.tokens,
			LastRefillUnix: b.lastRefill.Unix(),
		})
	}
	return Snapshot{
		Capacity:        rl.capacity,
		RefillPerSecond: rl.refillPerSecond,
		Denials:         rl.denials,
		BucketCount:     len(rl.buckets),
		Buckets:         out,
	}
}

// enforceRateLimit wraps the "per-IP token-bucket gate" pattern used by
// every mutating endpoint that opted into rate limiting. Returns true
// when the request is over-budget — in that case the response has
// already been written (429 + Retry-After) and the caller should
// return. Returns false (and writes nothing) when the request is
// within budget or when `rl` is nil (backwards-compatible no-op for
// older binaries that don't construct a limiter).
//
// `label` appears in the user-visible error body so an operator
// tailing logs / a developer in devtools can tell which endpoint
// fired the limiter (vs. a generic "too many requests").
//
// Use enforceRateLimitByOwner instead when the endpoint is
// owner-authenticated — sharing a residential IP shouldn't make
// roommates compete for the same per-user budget.
func enforceRateLimit(rl *RateLimiter, w http.ResponseWriter, r *http.Request, label string) bool {
	return enforceRateLimitByKey(rl, w, r, clientIP(r), label)
}

// enforceRateLimitByOwner is the per-user-account variant of
// enforceRateLimit. Buckets by X-HexDek-Owner so authenticated
// callers get individual budgets that follow them across devices and
// networks. Falls back to per-IP bucketing when the header is absent
// — a missing owner header must NOT bypass throttling (the upstream
// ownership check will still 403, but the limiter mustn't grant an
// attacker an effectively unlimited rate to drive that 403).
//
// Bucket keys are namespaced ("owner:" / "ip:") so an attacker can't
// collide their owner slug with a victim's IP to share the victim's
// budget refill.
func enforceRateLimitByOwner(rl *RateLimiter, w http.ResponseWriter, r *http.Request, label string) bool {
	return enforceRateLimitByKey(rl, w, r, ownerOrIPKey(r), label)
}

// enforceRateLimitDual gates a request through BOTH a per-IP and a
// per-owner token bucket — first denial wins. Use for expensive
// endpoints that need defense across two failure modes simultaneously:
//   - shared IP, many owners (NAT / coffeeshop / botnet exit node)
//     abusing the per-owner budget en masse → per-IP bucket throttles
//   - one owner across many IPs (stolen session, distributed scraper)
//     evading the per-IP bucket by churning source addresses → per-owner
//     bucket throttles
//
// Either limiter may be nil for backwards compatibility — a nil limiter
// is a no-op for that axis (so a binary that constructs only the per-IP
// limiter keeps working unchanged). When both are nil this whole call
// is a no-op, matching the existing single-limiter behavior.
//
// The per-IP gate reuses enforceRateLimit verbatim so its bucket-key
// shape (bare clientIP, no namespace prefix) stays bit-identical to
// every other per-IP call site — keeping the existing shared-budget
// contract (the import endpoints and the analyze endpoint share one
// DeckImportLimiter bucket per IP). The per-owner gate uses the
// ownerOrIPKey helper directly (NOT enforceRateLimitByOwner) so we
// can skip the IP-fallback when no owner header was sent — otherwise
// the per-owner axis would re-throttle the per-IP bucket on the same
// request, double-debiting tokens.
//
// X-RateLimit-By tags which axis denied (`ip` or `owner`) so
// operators tailing logs can tell them apart at a glance.
func enforceRateLimitDual(perIP, perOwner *RateLimiter, w http.ResponseWriter, r *http.Request, label string) bool {
	if perIP != nil {
		if enforceRateLimit(perIP, w, r, label) {
			w.Header().Set("X-RateLimit-By", "ip")
			return true
		}
	}
	if perOwner != nil {
		// Only key by owner when the header is actually present —
		// otherwise the owner bucket would re-throttle on the same IP
		// key the perIP bucket just charged, double-debiting a single
		// request. Per-IP-only callers stay on the per-IP-only path.
		if owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner"))); owner != "" {
			if enforceRateLimitByKey(perOwner, w, r, "owner:"+owner, label) {
				w.Header().Set("X-RateLimit-By", "owner")
				return true
			}
		}
	}
	return false
}

// enforceRateLimitByKey is the shared core. Callers compute the
// bucket key (per-IP, per-owner, per-API-token, ...) and hand it in.
// Centralizing the over-limit response shape (429 + Retry-After +
// labeled body) keeps the keying decision in one place per endpoint.
func enforceRateLimitByKey(rl *RateLimiter, w http.ResponseWriter, _ *http.Request, key, label string) bool {
	if rl == nil {
		return false
	}
	ok, retryAfter := rl.Allow(key)
	if ok {
		return false
	}
	secs := int(retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
	writeError(w, http.StatusTooManyRequests, label+" rate limit exceeded — too many requests")
	return true
}

// ownerOrIPKey derives the bucket key for per-user rate limits.
// Prefers the X-HexDek-Owner header (lowercased + trimmed); falls back
// to the client IP when the header is missing or blank. Keys are
// namespaced to prevent cross-pool collisions ("owner:alice" cannot
// share a bucket with the IP "alice", however unlikely that string
// match would be — defense in depth costs nothing here).
func ownerOrIPKey(r *http.Request) string {
	if owner := strings.ToLower(strings.TrimSpace(r.Header.Get("X-HexDek-Owner"))); owner != "" {
		return "owner:" + owner
	}
	return "ip:" + clientIP(r)
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
