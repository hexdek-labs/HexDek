package hexapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleTournamentStats implements GET /api/tournament/{id}/stats.
//
// {id} is the gauntlet_runs.run_id (autoincrement int). The handler
// looks up the run for its (deck_key, started_at, finished_at)
// window, then scans showmatch_game rows in that window where the
// deck participated. heimdall.ComputeTournamentStats aggregates the
// per-game rows into deck distribution / winrate variance / longest
// / fastest signals.
//
// Returns 404 when the run_id is unknown so the frontend can show
// "no such tournament" cleanly. Returns 503 when the DB isn't
// attached (tests / dev runs) — a 200 with empty stats would lie
// about a tournament's existence.
func (h *Handler) handleTournamentStats(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	runID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || runID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid tournament id")
		return
	}
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeError(w, http.StatusServiceUnavailable, "tournament store unavailable")
		return
	}

	run, err := db.LoadGauntletRunByID(r.Context(), h.Showmatch.sqlDB, runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "tournament not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "tournament lookup failed")
		return
	}

	rows, err := db.LoadTournamentGames(r.Context(), h.Showmatch.sqlDB,
		run.DeckKey, run.StartedAt.Unix(), run.FinishedAt.Unix())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tournament games query failed")
		return
	}

	games := make([]heimdall.TournamentGameInput, 0, len(rows))
	for _, row := range rows {
		games = append(games, heimdall.TournamentGameInput{
			GameID:     row.GameID,
			FinishedAt: row.FinishedAt,
			Turns:      row.Turns,
			Won:        row.Won,
			Draw:       row.Draw,
			Opponents:  row.Opponents,
		})
	}

	meta := heimdall.TournamentRunMeta{
		RunID:      run.RunID,
		DeckKey:    run.DeckKey,
		Commander:  run.Commander,
		StartedAt:  run.StartedAt.Unix(),
		FinishedAt: run.FinishedAt.Unix(),
		Games:      run.Games,
		Wins:       run.Wins,
		Losses:     run.Losses,
		WinRate:    run.WinRate,
		AvgTurns:   run.AvgTurns,
	}
	writeJSON(w, heimdall.ComputeTournamentStats(meta, games))
}
