// First-party frontend error telemetry — the error sibling of the
// Temporal Pincer pageview endpoint (telemetry_pincer.go). The frontend
// posts here from three producers: the React ErrorBoundary
// (componentDidCatch), the global window 'error' listener, and the
// 'unhandledrejection' listener (see hexdek/src/lib/errorTelemetry.js).
// Rows land next to the pageview rows so a crash correlates with the
// anon session that hit it.
//
// The client already filters chatter (resource-load errors, cross-origin
// script noise, plain connectivity failures), dedupes per message, and
// caps reports per session — this handler adds the server-side floor:
// body-size limit, field clamps, and a per-IP token bucket so a hostile
// or crash-looping client can't write unbounded rows.

package hexapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	maxErrMessageLen = 500
	maxErrStackLen   = 4096
	maxErrKindLen    = 32
	maxErrSourceLen  = 16
	maxErrBodyLen    = 16 * 1024 // message + 4 KB stack + envelope headroom
)

type errorTelemetryBody struct {
	AnonID    string `json:"anon_id"`
	Message   string `json:"message"`
	Stack     string `json:"stack"`
	Kind      string `json:"kind"`   // boundary classification: chunk | storage | render | uncaught | rejection
	Source    string `json:"source"` // producer: boundary | window | promise
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"` // unix ms; server falls back to now()
}

func (h *Handler) handleErrorTelemetry(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if enforceRateLimit(h.ErrorTelemetryLimiter, w, r, "error telemetry") {
		return
	}
	var body errorTelemetryBody
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(maxErrBodyLen))).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	msg := strings.TrimSpace(body.Message)
	if msg == "" {
		writeError(w, http.StatusBadRequest, "message required")
		return
	}
	// anon_id is optional here (unlike pageview): iOS private mode blocks
	// localStorage, and storage-blocked sessions are exactly the crash
	// class the r60 work chased — refusing their reports would blind us
	// to it. Invalid non-empty IDs are dropped to NULL rather than 400ing
	// so a malformed client still gets its error through.
	anonID := strings.TrimSpace(body.AnonID)
	if !validAnonID(anonID) {
		anonID = ""
	}
	ts := body.Timestamp
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO frontend_errors (anon_id, message, stack, kind, source, path, ts)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nullableStr(anonID),
		clampStr(msg, maxErrMessageLen),
		nullableStr(clampStr(strings.TrimSpace(body.Stack), maxErrStackLen)),
		clampStr(strings.TrimSpace(body.Kind), maxErrKindLen),
		clampStr(strings.TrimSpace(body.Source), maxErrSourceLen),
		clampStr(strings.TrimSpace(body.Path), maxPathLen),
		ts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
