package hexapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleGameReplay streams the persisted observation events for a
// game as NDJSON (newline-delimited JSON, one ReplayEvent per
// line). Routed at GET /api/games/{id}/replay.
//
// The endpoint requires a persisted observation snapshot — without
// one there's nothing chronological to stream. Callers that just
// need game metadata should hit /api/games/{id}/summary, which
// degrades gracefully to db_only.
//
// Response codes:
//   - 200 application/x-ndjson — one ReplayEvent JSON per line
//   - 400 — non-numeric / zero / negative game id
//   - 404 — game id unknown OR no snapshot persisted for the game
//   - 500 — DB error or malformed snapshot
//   - 503 — DB unavailable
//
// NDJSON was chosen over a single JSON array so very long replay
// logs can be consumed incrementally — the legality auditor UI is
// expected to read events line-by-line and update its timeline
// without buffering the entire log.
func (h *Handler) handleGameReplay(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid game id")
		return
	}

	ctx := r.Context()
	if _, err := db.LoadGameByID(ctx, h.db, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "game not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "load game: "+err.Error())
		return
	}

	// Same parse-cache as /api/games/{id}/summary (#202). /replay
	// keeps its stricter error contract: missing snapshot is a
	// 404 (vs /summary's db_only fallback), malformed is a 500
	// (vs /summary's graceful fallback). The cache layer is
	// transparent — it just skips the Load+Unmarshal pair on a
	// hit (~93μs/op of the baseline 109μs handler time).
	h.ensureSnapshotCache()
	snap, _, err := h.loadCachedSnapshot(ctx, id)
	switch {
	case err == nil:
		// fall through to render
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "no observation snapshot for game")
		return
	case IsMalformedSnapshot(err):
		log.Printf("game_replay: malformed snapshot for game %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "malformed snapshot")
		return
	default:
		writeError(w, http.StatusInternalServerError, "load observation: "+err.Error())
		return
	}

	events := heimdall.BuildReplayLog(snap)
	streamNDJSON(w, events)
}

// streamNDJSON writes one JSON-encoded ReplayEvent per line and
// flushes after every event when the underlying ResponseWriter
// supports http.Flusher. Caller has not written status/headers yet;
// this function is responsible for both. Errors mid-stream are
// logged and abort the stream (the client sees a truncated body —
// acceptable for a read-only log endpoint where retrying is cheap).
func streamNDJSON(w http.ResponseWriter, events []heimdall.ReplayEvent) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Disable proxy/caddy response buffering for the streaming case;
	// Caddy honors this header to passthrough chunked writes.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	flusher, canFlush := w.(http.Flusher)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			log.Printf("game_replay: encode event: %v", err)
			return
		}
		if canFlush {
			flusher.Flush()
		}
	}
}
