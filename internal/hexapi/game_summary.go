package hexapi

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleGameSummary returns the unified post-game GameSummary blob
// for the requested showmatch_game id. Routed at
// GET /api/games/{id}/summary.
//
// Today the heimdall observation pipeline (CommanderZoneVisits /
// RegretCards / MVPCards) is not backed by durable storage — those
// signals only exist in memory during a live replay. For historical
// games persisted in showmatch_game we therefore return the
// "db_only" variant of the summary: game metadata + a game_end
// turning point, with empty arrays for the three R60 signals.
//
// Once observation persistence lands, this handler is the natural
// place to hydrate the rich path; the heimdall.BuildGameSummary
// constructor already accepts a populated Observation, so the only
// change will be sourcing the Observation from a new table instead
// of falling back to a synthetic one.
func (h *Handler) handleGameSummary(w http.ResponseWriter, r *http.Request) {
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

	summary := buildDBOnlyGameSummary(game)
	writeJSON(w, summary)
}

// buildDBOnlyGameSummary constructs a heimdall.GameSummary from a
// persisted GameRecord. Pure function so the handler is trivially
// testable without spinning a router.
//
// The three R60 signal slices are non-nil-but-empty so JSON callers
// always see `"regret_cards": []` rather than `"regret_cards": null`
// — frontend code can iterate without guarding against nil.
func buildDBOnlyGameSummary(g db.GameRecord) heimdall.GameSummary {
	turn := g.Turns
	winner := g.Winner
	desc := fmt.Sprintf("seat %d won on turn %d", winner, turn)
	if winner < 0 {
		desc = fmt.Sprintf("game ended without a winner on turn %d", turn)
	}

	return heimdall.GameSummary{
		GameID:              strconv.FormatInt(g.GameID, 10),
		Winner:              winner,
		WinnerName:          g.WinnerName,
		Turns:               turn,
		EndReason:           g.EndReason,
		CommanderZoneVisits: []heimdall.CommanderZoneVisit{},
		RegretCards:         []heimdall.RegretCard{},
		MVPCards:            []heimdall.MVPCard{},
		TurningPoints: []heimdall.TurningPoint{{
			Turn:        turn,
			Kind:        "game_end",
			Seat:        winner,
			Description: desc,
		}},
		DataSource: "db_only",
	}
}
