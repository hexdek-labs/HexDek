package hexapi

import (
	"context"
	"net/http"
	"os"
	"os/exec"
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
// strict DB readiness should look at db_reachable / dependencies.scylla
// instead of HTTP status.
//
// Dependencies is the k8s-style per-component sub-status (added r60).
// Each value is "ok" or "fail" — operators can parse the object
// without converting Go-specific JSON shapes. db_reachable is kept
// alongside dependencies.scylla for backwards-compat: existing
// monitors that read the top-level boolean keep working.
type healthResponse struct {
	Status       string            `json:"status"`
	UptimeSec    int64             `json:"uptime_sec"`
	Version      string            `json:"version"`
	DBReachable  bool              `json:"db_reachable"`
	Dependencies map[string]string `json:"dependencies"`
}

// Dependency status values. Exported so monitors importing the
// package can switch on them rather than string-comparing literals.
const (
	HealthOK   = "ok"
	HealthFail = "fail"
)

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
	dbOK := h.pingDB(r.Context())
	resp := healthResponse{
		Status:      "ok",
		UptimeSec:   int64(time.Since(startedAt).Seconds()),
		Version:     Version,
		DBReachable: dbOK,
		Dependencies: map[string]string{
			"scylla": healthBool(dbOK),
			"freya":  healthBool(freyaBinaryAvailable()),
			"hat":    healthBool(h.hatReady()),
		},
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, resp)
}

// healthBool maps a bounded liveness probe result to the wire string.
func healthBool(ok bool) string {
	if ok {
		return HealthOK
	}
	return HealthFail
}

// freyaBinaryAvailable reports whether the hexdek-freya analysis
// subprocess is reachable. Mirrors the discovery path runFreya uses
// in handler.go: PATH first, then ./hexdek-freya relative to the
// current working directory. We don't try to invoke it (the cold-
// start cost would dwarf a healthcheck budget); presence on disk is
// the cheapest meaningful signal.
//
// Test/dev binaries that don't ship the freya binary report fail —
// the dashboard surfaces it; the server itself still serves requests.
func freyaBinaryAvailable() bool {
	if _, err := exec.LookPath("hexdek-freya"); err == nil {
		return true
	}
	if _, err := os.Stat("./hexdek-freya"); err == nil {
		return true
	}
	return false
}

// hatReady reports whether the in-process YggdrasilHat AI player is
// usable. HAT is not a separate binary — it's the AI package that
// drives Showmatch games. It needs the AST corpus + MetaDB loaded,
// both of which the Showmatch background loader populates at startup.
// "fail" here means either Showmatch wasn't constructed (test/dev
// binary) OR the corpus load hasn't finished yet (still warming).
func (h *Handler) hatReady() bool {
	if h.Showmatch == nil {
		return false
	}
	return h.Showmatch.DeckParserMeta() != nil
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
