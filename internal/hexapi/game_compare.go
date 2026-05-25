package hexapi

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleGameCompare returns a side-by-side comparison of two completed
// games. Routed at GET /api/games/compare?a={id}&b={id}.
//
// Both games are loaded via the same snapshot-cache → db-only fallback
// chain that /api/games/{id}/summary uses, so the comparison embeds
// "rich" summaries when observation snapshots exist and degrades to
// "db_only" summaries for older games (MVPDiff is empty in that case,
// but outcome / turns / shared-deck diffs still produce useful output).
//
// Errors:
//   - 400 if a / b query params are missing, non-numeric, ≤0, or equal
//   - 404 if either game id is unknown
//   - 503 if the database isn't wired (test-handler / boot-time edge)
func (h *Handler) handleGameCompare(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	idA, errA := parseCompareID(r.URL.Query().Get("a"))
	idB, errB := parseCompareID(r.URL.Query().Get("b"))
	if errA != nil {
		writeError(w, http.StatusBadRequest, "invalid game id 'a': "+errA.Error())
		return
	}
	if errB != nil {
		writeError(w, http.StatusBadRequest, "invalid game id 'b': "+errB.Error())
		return
	}
	if idA == idB {
		writeError(w, http.StatusBadRequest, "'a' and 'b' must reference different games")
		return
	}

	ctx := r.Context()
	summaryA, status, err := h.loadGameSummaryByID(ctx, idA)
	if err != nil {
		writeError(w, status, "game a: "+err.Error())
		return
	}
	summaryB, status, err := h.loadGameSummaryByID(ctx, idB)
	if err != nil {
		writeError(w, status, "game b: "+err.Error())
		return
	}

	writeJSON(w, heimdall.CompareGames(summaryA, summaryB))
}

// loadGameSummaryByID mirrors handleGameSummary's load logic: try the
// cached snapshot for a "rich" summary, fall back to a db-only
// summary if no snapshot exists, return the underlying error
// (with an HTTP status hint) otherwise.
//
// Extracted so handleGameCompare doesn't have to duplicate the four-
// branch load chain. Returns (summary, httpStatus, error); on success
// the status is http.StatusOK and the error is nil.
func (h *Handler) loadGameSummaryByID(ctx context.Context, id int64) (heimdall.GameSummary, int, error) {
	game, err := db.LoadGameByID(ctx, h.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		return heimdall.GameSummary{}, http.StatusNotFound, errors.New("game not found")
	}
	if err != nil {
		return heimdall.GameSummary{}, http.StatusInternalServerError, err
	}

	h.ensureSnapshotCache()
	snap, _, err := h.loadCachedSnapshot(ctx, id)
	if err == nil {
		summary := heimdall.BuildGameSummaryFromSnapshot(snap, game.EndReason)
		if summary.Winner != game.Winner {
			summary.Winner = game.Winner
		}
		summary.WinnerName = game.WinnerName
		if game.Turns > summary.Turns {
			summary.Turns = game.Turns
		}
		return summary, http.StatusOK, nil
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// no snapshot → fall through to db_only
	case IsMalformedSnapshot(err):
		log.Printf("game_compare: malformed snapshot for game %d: %v", id, err)
	default:
		return heimdall.GameSummary{}, http.StatusInternalServerError, err
	}
	return buildDBOnlyGameSummary(game), http.StatusOK, nil
}

func parseCompareID(s string) (int64, error) {
	if s == "" {
		return 0, errors.New("missing")
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, errors.New("must be positive")
	}
	return id, nil
}
