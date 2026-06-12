package hexapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleMetaTrends implements GET /api/meta/trends.
//
// Query params:
//   - weeks (optional, default 4, max 26): rolling-window size in weeks
//
// Returns 200 + MetaTrends body. Empty Archetypes/BiggestGainers/
// BiggestLosers (but well-formed envelope) when no games exist in
// the window or the showmatch DB isn't wired — frontend renders the
// empty state without needing to distinguish "no data" from "error".
func (h *Handler) handleMetaTrends(w http.ResponseWriter, r *http.Request) {
	weeks := 4
	if v := strings.TrimSpace(r.URL.Query().Get("weeks")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 26 {
				n = 26
			}
			weeks = n
		}
	}

	refUnix := time.Now().Unix()
	sinceUnix := refUnix - int64(weeks)*7*24*3600

	sqlDB := h.cardStatsDB()
	if sqlDB == nil {
		writeJSON(w, heimdall.ComputeMetaArchetypeTrends(nil, refUnix, weeks))
		return
	}
	rows, err := db.LoadMetaSeatOutcomes(r.Context(), sqlDB, sinceUnix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "meta trends query failed")
		return
	}

	lookup := h.deckArchetypeLookup()
	games := make([]heimdall.MetaGameInput, 0, len(rows))
	for _, row := range rows {
		games = append(games, heimdall.MetaGameInput{
			FinishedAt: row.FinishedAt,
			Archetype:  lookup(row.DeckKey),
			Won:        row.Won,
		})
	}

	out := heimdall.ComputeMetaArchetypeTrends(games, refUnix, weeks)
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, out)
}
