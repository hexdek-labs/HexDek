package hexapi

import (
	"net/http"
	"strconv"

	"github.com/hexdek/hexdek/internal/db"
)

// handleDeckHistory returns per-game performance for one deck plus an
// aggregate-totals roll-up over the same window. Routed at
// GET /api/decks/{owner}/{id}/history?limit=N.
//
// Distinct from /elo-history: that endpoint returns gauntlet RUNS
// (rating snapshots at completion of an N-game gauntlet), this one
// returns individual GAMES. Distinct from /version-trends: that one
// buckets games into deck versions; this one is per-game without
// version-walking.
//
// Games are returned chronological-ascending so a frontend chart can
// plot win/loss-over-time without re-sorting (mirrors elo-history's
// convention). The totals block aggregates across the SAME window the
// games slice covers, so totals.games == len(games) always — useful
// because the limit cap means "totals" is a windowed view, not a
// lifetime stat (the deck_snapshot.record block carries the lifetime
// view from showmatch_elo).
//
// limit defaults to 50, capped at 500. limit > 500 is silently
// clamped — matches the cap pattern in handleDeckEloHistory + the
// other paged endpoints rather than 400-ing.
func (h *Handler) handleDeckHistory(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id

	limit := 50
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{
			"deck_key": deckKey,
			"totals":   emptyDeckHistoryTotals(),
			"games":    []any{},
		})
		return
	}

	rows, err := db.LoadDeckGameHistory(r.Context(), h.Showmatch.sqlDB, deckKey, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// DB returns newest-first; reverse for chronological display so
	// the games slice is naturally chart-plottable left-to-right.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}

	writeJSON(w, map[string]any{
		"deck_key": deckKey,
		"totals":   computeDeckHistoryTotals(rows),
		"games":    rows,
	})
}

// deckHistoryTotals is the windowed aggregate emitted alongside the
// per-game rows. Games / Wins / Losses / Draws sum to len(rows); the
// derived fields (WinRate, AvgTurns) are omitted via omitempty when
// the window is empty so clients can branch on field presence rather
// than treating 0.0 as "0% winrate over 0 games."
type deckHistoryTotals struct {
	Games    int     `json:"games"`
	Wins     int     `json:"wins"`
	Losses   int     `json:"losses"`
	Draws    int     `json:"draws"`
	WinRate  float64 `json:"win_rate,omitempty"`
	AvgTurns float64 `json:"avg_turns,omitempty"`
}

func emptyDeckHistoryTotals() deckHistoryTotals { return deckHistoryTotals{} }

func computeDeckHistoryTotals(rows []db.DeckGameHistoryRow) deckHistoryTotals {
	t := deckHistoryTotals{}
	if len(rows) == 0 {
		return t
	}
	turnSum := 0
	for _, r := range rows {
		t.Games++
		turnSum += r.Turns
		switch {
		case r.Draw:
			t.Draws++
		case r.Won:
			t.Wins++
		default:
			t.Losses++
		}
	}
	t.WinRate = float64(t.Wins) / float64(t.Games)
	t.AvgTurns = float64(turnSum) / float64(t.Games)
	return t
}
