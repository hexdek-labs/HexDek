package hexapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// handleGameSearch implements GET /api/games/search — a full-history
// search endpoint built around the "find all my games where I lost
// to a board wipe on turn 5" use case. Distinct from
// /api/games/summaries (which only walks games with persisted
// observation snapshots): this endpoint scans the full
// showmatch_game table and supports the owner-relative filters
// (won/lost from MY perspective) the summary archive lacks.
//
// Query params (all optional; combine with implicit AND):
//   - owner=<id>        games where this owner had a seat
//   - q=<substring>     case-insensitive substring against
//                       winner_name | end_reason | any seat's
//                       commander
//   - won=true|false    requires owner; filters to games owner won
//                       (true) or lost (false)
//   - min_turn=<n>      inclusive lower bound on g.turns
//   - max_turn=<n>      inclusive upper bound on g.turns
//   - end_reason=<s>    exact case-insensitive match on g.end_reason
//   - since=<unix>      finished_at lower bound (inclusive)
//   - until=<unix>      finished_at upper bound (inclusive)
//   - limit=<n>         default 50, capped at 500
//   - offset=<n>        pagination offset
//
// Response shape:
//
//	{
//	  "rows":   [GameSearchRow, ...],
//	  "count":  N,
//	  "limit":  L,
//	  "offset": O,
//	  "owner":  "..."     // echoed back when set
//	}
//
// rows[].owner_seat is -1 when no owner filter was supplied. When
// owner is set, rows[].owner_won == (owner_seat == winner).
func (h *Handler) handleGameSearch(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	q := r.URL.Query()
	filters := db.GameSearchFilters{
		Owner:     strings.ToLower(strings.TrimSpace(q.Get("owner"))),
		Q:         strings.TrimSpace(q.Get("q")),
		EndReason: strings.TrimSpace(q.Get("end_reason")),
	}
	if filters.Owner != "" && !validatePathComponent(filters.Owner) {
		writeError(w, http.StatusBadRequest, "invalid owner")
		return
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
	if m := q.Get("min_turn"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			filters.MinTurn = v
		}
	}
	if m := q.Get("max_turn"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v > 0 {
			filters.MaxTurn = v
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
	if won := q.Get("won"); won != "" {
		if filters.Owner == "" {
			writeError(w, http.StatusBadRequest, "won filter requires owner")
			return
		}
		switch strings.ToLower(won) {
		case "true", "1", "yes":
			t := true
			filters.Won = &t
		case "false", "0", "no":
			f := false
			filters.Won = &f
		default:
			writeError(w, http.StatusBadRequest, "won must be true or false")
			return
		}
	}

	rows, err := db.SearchGames(r.Context(), h.db, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search: "+err.Error())
		return
	}
	if rows == nil {
		rows = []db.GameSearchRow{}
	}

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
		"owner":  filters.Owner,
	})
}
