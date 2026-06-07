package hexapi

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

// freyaBinaryName is the platform-specific filename of the Freya
// analysis binary — hexdek-freya, plus the .exe suffix on Windows.
func freyaBinaryName() string {
	if runtime.GOOS == "windows" {
		return "hexdek-freya.exe"
	}
	return "hexdek-freya"
}

// findFreyaBinary locates the hexdek-freya analysis binary, returning
// an exec-ready path and whether it was found. Discovery order, most
// specific first:
//
//  1. alongside the running server executable (the canonical deploy
//     shape — server + freya are built into the same directory), using
//     the platform extension. This is the ONLY branch that works on
//     Windows: the binary is hexdek-freya.exe, which neither a
//     bare-name os.Stat("./hexdek-freya") nor (since Go 1.19) a cwd
//     LookPath would resolve.
//  2. on PATH — LookPath applies PATHEXT, so the bare name resolves
//     hexdek-freya.exe on Windows / hexdek-freya elsewhere.
//  3. in the current working directory, returned with a leading "./"
//     so exec.Command treats it as a relative path rather than re-
//     triggering a PATH search (Go 1.19+ refuses to run a cwd match
//     found via a separator-less name).
//
// Both runFreya and freyaBinaryAvailable route through this so the
// health probe can never disagree with the path runFreya will execute.
func findFreyaBinary() (string, bool) {
	name := freyaBinaryName()
	if exe, err := os.Executable(); err == nil {
		cand := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(cand); err == nil {
			return cand, true
		}
	}
	if p, err := exec.LookPath("hexdek-freya"); err == nil {
		return p, true
	}
	if _, err := os.Stat(name); err == nil {
		return "." + string(os.PathSeparator) + name, true
	}
	return "", false
}

// freyaBinaryAvailable reports whether the hexdek-freya analysis
// subprocess is reachable. Delegates to findFreyaBinary so the health
// probe matches runFreya's discovery exactly. We don't invoke the
// binary (the cold-start cost would dwarf a healthcheck budget);
// presence on disk is the cheapest meaningful signal.
//
// Test/dev binaries that don't ship the freya binary report fail —
// the dashboard surfaces it; the server itself still serves requests.
func freyaBinaryAvailable() bool {
	_, ok := findFreyaBinary()
	return ok
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
