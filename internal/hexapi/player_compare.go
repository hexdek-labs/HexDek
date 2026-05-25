package hexapi

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

const (
	playerCompareDefaultWindow      = 30
	playerCompareMaxWindow          = 200
	playerCompareDefaultRecentSlice = 10
	playerCompareMaxRecentSlice     = 100
	playerCompareHeadToHeadLimit    = 100
)

// handlePlayerCompare implements GET /api/players/compare.
//
// Query params:
//   - a (required): player A's id
//   - b (required, must differ from a): player B's id
//   - window (optional, default 30, max 200): rolling-window size for
//     the per-player PlayerTrends summary
//   - recent_window (optional, default 10, max 100): sub-window for
//     rating velocity's "recent" half — when smaller than `window`,
//     BuildRatingVelocityFromHalves produces a meaningful direction
//
// Returns 200 + PlayerComparison body. Errors:
//   - 400 on missing / invalid id, or a == b
//   - 503 when the database isn't wired
func (h *Handler) handlePlayerCompare(w http.ResponseWriter, r *http.Request) {
	idA := strings.TrimSpace(r.URL.Query().Get("a"))
	idB := strings.TrimSpace(r.URL.Query().Get("b"))
	if idA == "" || !validatePathComponent(idA) {
		writeError(w, http.StatusBadRequest, "invalid player id 'a'")
		return
	}
	if idB == "" || !validatePathComponent(idB) {
		writeError(w, http.StatusBadRequest, "invalid player id 'b'")
		return
	}
	playerA := strings.ToLower(idA)
	playerB := strings.ToLower(idB)
	if playerA == playerB {
		writeError(w, http.StatusBadRequest, "'a' and 'b' must reference different players")
		return
	}

	window := parseClamped(r.URL.Query().Get("window"), playerCompareDefaultWindow, 1, playerCompareMaxWindow)
	recentWindow := parseClamped(r.URL.Query().Get("recent_window"), playerCompareDefaultRecentSlice, 1, playerCompareMaxRecentSlice)
	if recentWindow > window {
		recentWindow = window
	}

	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	sqlDB := h.Showmatch.sqlDB

	lookup := h.deckArchetypeLookup()

	trendsA, recentA, err := loadComparisonSides(r.Context(), sqlDB, playerA, window, recentWindow, lookup)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "player a load failed")
		return
	}
	trendsB, recentB, err := loadComparisonSides(r.Context(), sqlDB, playerB, window, recentWindow, lookup)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "player b load failed")
		return
	}

	headToHead, err := db.LoadPlayerHeadToHead(r.Context(), sqlDB, playerA, playerB, playerCompareHeadToHeadLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "head-to-head query failed")
		return
	}
	h2h := make([]heimdall.HeadToHeadGameRecord, 0, len(headToHead))
	for _, g := range headToHead {
		h2h = append(h2h, heimdall.HeadToHeadGameRecord{
			FinishedAt: g.FinishedAt,
			Winner:     g.Winner,
			SeatA:      g.SeatA,
			SeatB:      g.SeatB,
		})
	}

	out := heimdall.ComparePlayers(heimdall.PlayerComparisonInput{
		PlayerA:              trendsA,
		PlayerB:              trendsB,
		HeadToHead:           h2h,
		RatingVelocityRecent: recentWindow,
	})
	// Replace the placeholder velocity rows with the richer
	// recent-vs-overall ones built from the second PlayerTrends fetch.
	out.RatingVelocityA = heimdall.BuildRatingVelocityFromHalves(
		recentA.GamesPlayed, recentA.Wins, trendsA.GamesPlayed, trendsA.Wins)
	out.RatingVelocityB = heimdall.BuildRatingVelocityFromHalves(
		recentB.GamesPlayed, recentB.Wins, trendsB.GamesPlayed, trendsB.Wins)

	writeJSON(w, out)
}

// loadComparisonSides returns (overall, recent) PlayerTrends for one
// player. We pull the larger window once and slice off the first
// recentN games for the recent sub-window — only one DB round-trip
// per player.
func loadComparisonSides(ctx context.Context, sqlDB *sql.DB, player string, window, recentN int, lookup func(string) string) (heimdall.PlayerTrends, heimdall.PlayerTrends, error) {
	rows, err := db.LoadOwnerGames(ctx, sqlDB, player, window)
	if err != nil {
		return heimdall.PlayerTrends{}, heimdall.PlayerTrends{}, err
	}
	games := make([]heimdall.PlayerGameInput, 0, len(rows))
	for _, row := range rows {
		games = append(games, heimdall.PlayerGameInput{
			Won:           row.Winner == row.MySeat,
			Draw:          row.Winner < 0,
			DeckKey:       row.MyDeckKey,
			OpponentCmdrs: row.Opponents,
		})
	}
	overall := heimdall.ComputePlayerTrends(player, window, games, lookup)

	recentGames := games
	if recentN > 0 && recentN < len(games) {
		recentGames = games[:recentN]
	}
	recent := heimdall.ComputePlayerTrends(player, recentN, recentGames, lookup)
	return overall, recent, nil
}

// parseClamped reads an integer from a query string, falls back to
// dflt when missing / invalid, and clamps the result to [min, max].
func parseClamped(raw string, dflt, min, max int) int {
	if raw == "" {
		return dflt
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return dflt
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

