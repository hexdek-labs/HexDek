package hexapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

// handleMetaEvolution implements GET /api/meta/evolution.
//
// Returns a MetaEvolution blob: per-archetype week-by-week play-rate
// + win-rate timeline plus pre-computed "biggest movers" lists for
// each dimension. Distinct from /api/meta/trends (winrate-only):
// this endpoint adds the play-rate axis so the meta dashboard can
// show which archetypes are growing/shrinking in popularity AND
// climbing/falling in winrate.
//
// Query params:
//   - weeks (optional, default 4, max 26): rolling-window size
//
// Empty data degrades to a well-formed envelope with non-nil empty
// slices; no 404 for "no games yet".
func (h *Handler) handleMetaEvolution(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, heimdall.ComputeMetaEvolution(nil, refUnix, weeks))
		return
	}
	rows, err := db.LoadMetaSeatOutcomes(r.Context(), sqlDB, sinceUnix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "meta evolution query failed")
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

	out := heimdall.ComputeMetaEvolution(games, refUnix, weeks)
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, out)
}
