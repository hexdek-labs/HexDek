package hexapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleGameSummary returns the unified post-game GameSummary blob
// for the requested showmatch_game id. Routed at
// GET /api/games/{id}/summary.
//
// Two paths:
//
//  1. "rich" — if showmatch_game_observation has a persisted
//     snapshot for this game, decode it and return the full
//     GameSummary (all three R60 signal arrays + commander-first-
//     cast turning points). This is the historical-game path that
//     #177 unlocks: observation snapshots are written once after a
//     game ends and re-read here on demand without replaying.
//  2. "db_only" — fall back to game metadata + a single game_end
//     turning point when no observation snapshot exists (older
//     games predating the snapshot table, or games where the
//     persistence sink wasn't wired).
//
// Both paths share the same response shape; the DataSource field
// discriminates so frontends can show "rich" or "limited" badges.
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

	// Load the per-seat scoreboard once and stamp it onto whichever
	// path wins below. A missing-rows / error case is non-fatal — the
	// summary still returns with an empty Seats slice rather than
	// 500ing the whole endpoint when one historical game is missing
	// its seat persistence.
	seats := loadSeatFinalStates(ctx, h.db, id)

	// Rich path: try the cached parsed snapshot first. Cache lookup
	// skips both the payload SELECT and the JSON unmarshal on a hit
	// (~125μs/op savings vs the un-cached path).
	h.ensureSnapshotCache()
	snap, _, err := h.loadCachedSnapshot(ctx, id)
	if err == nil {
		summary := heimdall.BuildGameSummaryFromSnapshot(snap, game.EndReason)
		// Snapshots predating game-end stamping may leave Winner
		// at the snapshot-time value; the GameRecord is the
		// authoritative end-state. WinnerName is DB-only.
		if summary.Winner != game.Winner {
			summary.Winner = game.Winner
		}
		summary.WinnerName = game.WinnerName
		if game.Turns > summary.Turns {
			summary.Turns = game.Turns
		}
		summary.Seats = seats
		writeJSON(w, summary)
		return
	}
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// No snapshot persisted — fall through to db_only.
	case IsMalformedSnapshot(err):
		// Bad payload — log and fall through to db_only so the
		// endpoint never hard-fails on a corrupt row.
		log.Printf("game_summary: malformed snapshot for game %d: %v", id, err)
	default:
		// Real DB error (connection lost, scan failure, etc.) —
		// surface as 500. Same contract as before the cache.
		writeError(w, http.StatusInternalServerError, "load observation: "+err.Error())
		return
	}

	// Fallback: db_only summary built from GameRecord metadata alone.
	summary := buildDBOnlyGameSummary(game)
	summary.Seats = seats
	writeJSON(w, summary)
}

// loadSeatFinalStates fetches the per-seat scoreboard rows for gameID
// and projects them into the public heimdall.SeatFinalState shape used
// in GameSummary.Seats. Soft-fails to an empty slice on any error so
// the summary endpoint never hard-fails when the seat persistence is
// missing for a single game — the rest of the summary still serves.
// A real DB-connection failure here is shadowed by the same error
// happening on the LoadGameByID call above, so logging is sufficient.
func loadSeatFinalStates(ctx context.Context, sqlDB *sql.DB, gameID int64) []heimdall.SeatFinalState {
	rows, err := db.LoadGameSeats(ctx, sqlDB, gameID)
	if err != nil {
		log.Printf("game_summary: load seats for game %d: %v", gameID, err)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	out := make([]heimdall.SeatFinalState, 0, len(rows))
	for _, r := range rows {
		out = append(out, heimdall.SeatFinalState{
			Seat:            r.Seat,
			Commander:       r.Commander,
			Life:            r.Life,
			HandSize:        r.HandSize,
			LibrarySize:     r.LibrarySize,
			GraveyardSize:   r.GYSize,
			BattlefieldSize: r.BFSize,
			Lost:            r.Lost,
		})
	}
	return out
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
