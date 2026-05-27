package hexapi

import (
	"context"
	"net/http"
	"time"
)

// Version is the process version string surfaced by /api/health.
// Defaults to "dev"; production builds override via:
//
//	go build -ldflags "-X github.com/hexdek/hexdek/internal/hexapi.Version=$(git describe --tags --dirty)"
//
// Kept as a package-level var (not a const) so the linker flag can
// rewrite it without a build-tagged file per environment.
var Version = "dev"

// startedAt records when the health endpoint first observed the
// process. Initialized lazily by handleHealth (and by Register, so
// uptime is anchored to the moment routes were mounted rather than
// the first request). Package-level so a Handler that's reconstructed
// between requests still reports the original boot time.
var startedAt = time.Now()

// healthDBPingTimeout caps the per-request DB liveness probe.
// Returning db_reachable=false within ~500ms is far more useful for
// orchestrators than blocking on a hung DB for the request timeout.
const healthDBPingTimeout = 500 * time.Millisecond

// healthResponse is the JSON shape returned by GET /api/health.
// Field names use snake_case to match the rest of hexapi's response
// schemas (uptime_sec, db_reachable). Status is "ok" when the
// process is healthy enough to serve requests — the DB-unreachable
// case still returns 200 so a transient DB blip doesn't fail the
// healthcheck and trigger a restart loop; orchestrators that need
// strict DB readiness should look at db_reachable instead of HTTP
// status.
type healthResponse struct {
	Status      string `json:"status"`
	UptimeSec   int64  `json:"uptime_sec"`
	Version     string `json:"version"`
	DBReachable bool   `json:"db_reachable"`
}

// handleHealth implements GET /api/health.
//
// Always returns 200 with a JSON body. The status field is "ok" for
// all served requests (a request that reached the handler means the
// HTTP server is alive). db_reachable is a separate boolean so a
// monitor can distinguish "process up but DB down" from "process
// down entirely" without parsing /health text. Uptime_sec is wall-
// clock seconds since first observation (Register or first request,
// whichever happened first).
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:      "ok",
		UptimeSec:   int64(time.Since(startedAt).Seconds()),
		Version:     Version,
		DBReachable: h.pingDB(r.Context()),
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, resp)
}

// pingDB performs a bounded liveness probe on the configured DB.
// Returns false on nil DB (tests / dev runs without persistence) or
// on any ping error within the timeout. Never propagates the error —
// callers only care about the boolean.
func (h *Handler) pingDB(parent context.Context) bool {
	if h.db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(parent, healthDBPingTimeout)
	defer cancel()
	return h.db.PingContext(ctx) == nil
}
