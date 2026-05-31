package hexapi

import (
	"net/http"
	"sort"
	"sync"
	"time"
)

// ratelimit_dashboard_r60.go — GET /api/admin/ratelimit returns the
// current bucket states of every registered limiter plus lifetime
// denial counts. Ops debugging surface: "is X getting throttled,
// which IPs are hot, has the gauntlet limiter caught any bursts
// today" — answerable without grepping logs.
//
// Auth: reuses adminAnomalyAuth (HEXDEK_ADMIN_OWNER env + matching
// X-HexDek-Owner header). Pre-existing admin gate; no localhost
// fallback by design (CWE-290 — see admin_anomalies.go docstring).
//
// Cardinality safeguard: every limiter is capped at 200 returned
// buckets via the maxDashboardBuckets constant. A limiter with
// thousands of unique IPs would otherwise blow up the response
// body; the trimmed window is the lexicographically-first 200 keys
// (deterministic across calls so refreshes show the same "hot
// list" until a new IP sorts in).

// maxDashboardBuckets caps per-limiter bucket count in the response.
// Picked to fit a typical dashboard table without paginating; raise
// when an operator legitimately needs the full set (rare — denials +
// bucket_count headline numbers cover most debug questions on their
// own).
const maxDashboardBuckets = 200

// LimiterEntry is one row in the dashboard response. Name is the
// human-readable label registered via RateLimitDashboard.Register
// — usually the same label the limiter publishes in its 429 body
// ("deck import", "gauntlet start", "spectator lineage", ...).
type LimiterEntry struct {
	Name     string   `json:"name"`
	Snapshot Snapshot `json:"snapshot"`
}

// RateLimitDashboard collects a labeled set of *RateLimiter pointers
// and serves their state via /api/admin/ratelimit. cmd/hexdek-server
// constructs one of these at startup and registers every limiter it
// wires onto handlers — the dashboard then iterates them in
// registration order to render the response.
//
// Zero value is usable; Register is the only mutator. Reads (Snapshot)
// take the registry lock briefly then release before touching the
// individual limiters, so a slow snapshot on one limiter doesn't
// block a Register call for another.
type RateLimitDashboard struct {
	mu       sync.Mutex
	entries  []dashboardEntry
}

type dashboardEntry struct {
	name    string
	limiter *RateLimiter
}

// Register adds a limiter to the dashboard under the given label.
// nil limiters are silently dropped so callers can pass optional
// fields ("if !flag, this is nil") without conditionally registering.
// Duplicate labels are allowed — the dashboard just renders both
// (operator can disambiguate by registration order).
func (d *RateLimitDashboard) Register(name string, rl *RateLimiter) {
	if rl == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries = append(d.entries, dashboardEntry{name: name, limiter: rl})
}

// snapshot returns a stable rendering of every registered limiter.
// Snapshots are taken outside the registry lock (each limiter does
// its own internal locking in SnapshotState) so a long-running snap
// on one limiter doesn't block a Register call for another.
func (d *RateLimitDashboard) snapshot() []LimiterEntry {
	d.mu.Lock()
	entries := make([]dashboardEntry, len(d.entries))
	copy(entries, d.entries)
	d.mu.Unlock()

	out := make([]LimiterEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, LimiterEntry{
			Name:     e.name,
			Snapshot: e.limiter.SnapshotState(maxDashboardBuckets),
		})
	}
	return out
}

// HandleRateLimitDashboard returns the GET /api/admin/ratelimit
// handler. Admin-only via adminAnomalyAuth (same gate as
// /api/admin/anomalies and /api/admin/conviction-events — see
// admin_anomalies.go for the env-var contract). Unauthorized
// callers get the unified ErrorResponse 403.
//
// Response shape:
//
//	{
//	  "limiters": [
//	    {
//	      "name": "deck import",
//	      "snapshot": {
//	        "capacity": 10,
//	        "refill_per_sec": 0.033,
//	        "denials": 47,
//	        "bucket_count": 12,
//	        "buckets": [
//	          {"key": "203.0.113.7", "tokens": 2.7, "last_refill_unix": ...}
//	        ]
//	      }
//	    }
//	  ],
//	  "captured_at_unix": 1735689600
//	}
//
// captured_at_unix lets clients diff snapshots over time to compute
// per-minute denial rates client-side without needing a /metrics
// pull alongside.
func HandleRateLimitDashboard(d *RateLimitDashboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !adminAnomalyAuth(r) {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		if d == nil {
			writeError(w, http.StatusServiceUnavailable, "ratelimit dashboard not configured")
			return
		}
		limiters := d.snapshot()
		// Sort by name for stable rendering — registration order is
		// the wirer's prerogative, but the operator UI benefits from
		// alphabetical so the same row sits in the same place across
		// reloads.
		sort.Slice(limiters, func(i, j int) bool {
			if limiters[i].Name != limiters[j].Name {
				return limiters[i].Name < limiters[j].Name
			}
			return false
		})
		writeJSON(w, map[string]any{
			"limiters":         limiters,
			"captured_at_unix": time.Now().Unix(),
		})
	}
}

// RegisterRateLimitDashboard mounts GET /api/admin/ratelimit on mux.
// Idempotent — safe to call from any registration block.
func RegisterRateLimitDashboard(mux *http.ServeMux, d *RateLimitDashboard) {
	mux.HandleFunc("GET /api/admin/ratelimit", HandleRateLimitDashboard(d))
}
