package hexapi

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleGameSummaryPDF serves the GameSummary as a downloadable
// PDF. Routed at GET /api/games/{id}/summary.pdf.
//
// Reuses the same load path as the JSON /summary endpoint —
// prefers the rich snapshot when persisted, falls back to a
// db_only summary otherwise — then renders via
// heimdall.RenderGameSummaryPDF.
//
// Response codes:
//   - 200 application/pdf — Content-Disposition attachment so the
//     browser triggers a download dialog rather than rendering
//     inline (better default for "I want to share this outside
//     hexdek.dev").
//   - 400 / 404 / 500 / 503 — same error contract as
//     /api/games/{id}/summary.
func (h *Handler) handleGameSummaryPDF(w http.ResponseWriter, r *http.Request) {
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
	game, err := db.LoadGameByID(ctx, h.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load game: "+err.Error())
		return
	}

	// Mirror the /summary handler: cached parsed snapshot first,
	// fall back to db_only on miss / malformed payload, GameRecord
	// overlay for authoritative end-state.
	h.ensureSnapshotCache()
	var summary heimdall.GameSummary
	snap, _, err := h.loadCachedSnapshot(ctx, id)
	switch {
	case err == nil:
		summary = heimdall.BuildGameSummaryFromSnapshot(snap, game.EndReason)
		summary.Winner = game.Winner
		summary.WinnerName = game.WinnerName
		if game.Turns > summary.Turns {
			summary.Turns = game.Turns
		}
	case errors.Is(err, sql.ErrNoRows):
		summary = buildDBOnlyGameSummary(game)
	case IsMalformedSnapshot(err):
		log.Printf("game_summary_pdf: malformed snapshot for game %d: %v", id, err)
		summary = buildDBOnlyGameSummary(game)
	default:
		writeError(w, http.StatusInternalServerError, "load observation: "+err.Error())
		return
	}

	body := heimdall.RenderGameSummaryPDF(summary)

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="hexdek-game-%d-summary.pdf"`, id))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write(body); werr != nil {
		log.Printf("game_summary_pdf: write body for game %d: %v", id, werr)
	}
}
