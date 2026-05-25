// Temporal Pincer — anonymous pageview tracking that stitches to an
// authenticated owner on login. The frontend generates a UUID per
// browser, posts every route change to /api/telemetry/pageview, and
// posts /api/telemetry/stitch the first time auth resolves. The stitch
// handler back-fills owner on every prior pageview row matching the
// anon_id, giving us continuous activity history across the auth event
// without storing any PII.

package hexapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// anonIDPattern matches a 36-char canonical UUID (8-4-4-4-12 hex with
// dashes). We're strict to keep junk out of the table; the frontend's
// crypto.randomUUID and our fallback both produce this shape.
var (
	anonIDLen   = 36
	maxPathLen  = 512
	maxReferLen = 512
	maxBodyLen  = 4 * 1024 // pageview/stitch payloads are tiny
)

func validAnonID(s string) bool {
	if len(s) != anonIDLen {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func clampStr(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}

type pageviewBody struct {
	AnonID    string `json:"anon_id"`
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"` // unix ms; server falls back to now()
	Referrer  string `json:"referrer"`
}

func (h *Handler) handlePageview(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body pageviewBody
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(maxBodyLen))).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if !validAnonID(body.AnonID) {
		writeError(w, http.StatusBadRequest, "anon_id must be a UUID")
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	path = clampStr(path, maxPathLen)
	referrer := clampStr(strings.TrimSpace(body.Referrer), maxReferLen)
	ts := body.Timestamp
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	// owner is filled lazily by handleStitch; record nil here.
	_, err := h.db.ExecContext(r.Context(),
		`INSERT INTO pageviews (anon_id, owner, path, ts, referrer) VALUES (?, NULL, ?, ?, ?)`,
		body.AnonID, path, ts, nullableStr(referrer))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "insert failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type stitchBody struct {
	AnonID string `json:"anon_id"`
	Owner  string `json:"owner"`
}

func (h *Handler) handleStitch(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var body stitchBody
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(maxBodyLen))).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if !validAnonID(body.AnonID) {
		writeError(w, http.StatusBadRequest, "anon_id must be a UUID")
		return
	}
	owner := strings.ToLower(strings.TrimSpace(body.Owner))
	if owner == "" || len(owner) > 64 {
		writeError(w, http.StatusBadRequest, "owner required")
		return
	}
	now := time.Now().UnixMilli()

	tx, err := h.db.BeginTx(r.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tx failed")
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(r.Context(),
		`INSERT OR REPLACE INTO session_stitch (anon_id, owner, stitched_at) VALUES (?, ?, ?)`,
		body.AnonID, owner, now); err != nil {
		writeError(w, http.StatusInternalServerError, "stitch failed")
		return
	}
	res, err := tx.ExecContext(r.Context(),
		`UPDATE pageviews SET owner = ? WHERE anon_id = ? AND owner IS NULL`,
		owner, body.AnonID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backfill failed")
		return
	}
	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "commit failed")
		return
	}
	backfilled, _ := res.RowsAffected()
	writeJSON(w, map[string]any{
		"stitched":    true,
		"anon_id":     body.AnonID,
		"owner":       owner,
		"backfilled":  backfilled,
		"stitched_at": now,
	})
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
