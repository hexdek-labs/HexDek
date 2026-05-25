package hexapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// handleGameSummaryArchive returns the searchable archive of
// persisted GameSummaries — every game in showmatch_game that has
// a corresponding showmatch_game_observation row. Routed at
// GET /api/games/summaries.
//
// Query parameters (all optional):
//   - since=<unix>  finished_at lower bound (inclusive)
//   - until=<unix>  finished_at upper bound (inclusive)
//   - commander=<s> substring against showmatch_game_seat.commander
//   - deck=<s>      substring against showmatch_game_seat.deck_key
//   - winner=<s>    substring against showmatch_game.winner_name
//   - limit=<n>     default 50, capped at 500 (defense in depth —
//                   db.SearchGameSummaries also caps, but echoing
//                   the cap here keeps the API contract explicit)
//   - offset=<n>    skip the first n rows for simple paging
//
// Response shape:
//
//	{
//	  "rows":   [SummaryArchiveRow, ...],
//	  "count":  N,
//	  "limit":  L,
//	  "offset": O
//	}
//
// rows is always non-nil so frontends can iterate without nil-
// guarding. The endpoint never paginates implicitly — callers
// supply limit+offset and decide their own paging UX.
func (h *Handler) handleGameSummaryArchive(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	q := r.URL.Query()
	filters := db.SummaryArchiveFilters{
		Commander: strings.TrimSpace(q.Get("commander")),
		Deck:      strings.TrimSpace(q.Get("deck")),
		Winner:    strings.TrimSpace(q.Get("winner")),
	}
	if s := q.Get("since"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v >= 0 {
			filters.Since = v
		}
	}
	if u := q.Get("until"); u != "" {
		if v, err := strconv.ParseInt(u, 10, 64); err == nil && v >= 0 {
			filters.Until = v
		}
	}
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			filters.Limit = v
		}
	}
	if o := q.Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			filters.Offset = v
		}
	}

	rows, err := db.SearchGameSummaries(r.Context(), h.db, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search: "+err.Error())
		return
	}
	if rows == nil {
		rows = []db.SummaryArchiveRow{}
	}

	// Mirror the effective limit + offset back to the client so
	// pagers can read them off the response without parsing the
	// request URL.
	effLimit := filters.Limit
	if effLimit <= 0 {
		effLimit = 50
	}
	if effLimit > 500 {
		effLimit = 500
	}
	effOffset := filters.Offset
	if effOffset < 0 {
		effOffset = 0
	}

	writeJSON(w, map[string]any{
		"rows":   rows,
		"count":  len(rows),
		"limit":  effLimit,
		"offset": effOffset,
	})
}
