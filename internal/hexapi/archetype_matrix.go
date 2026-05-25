package hexapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

const (
	archetypeMatrixDefaultWeeks = 12
	archetypeMatrixMaxWeeks     = 52
	// archetypeMatrixGameCap bounds the per-request DB pull. 50k
	// games × ~4 seats each → ~200k row scan + an in-memory matrix
	// of ~484 cells. The matrix itself is bounded by the archetype
	// list size, not game count, so this cap is purely a query-size
	// safety net.
	archetypeMatrixGameCap = 50000
)

// handleArchetypeMatrix implements GET /api/meta/archetype-vs-archetype.
//
// Returns the full 22×22 winrate matrix (using
// heimdall.CanonicalArchetypes) computed from games whose finished_at
// falls within the requested rolling window.
//
// Query params:
//   - weeks (optional, default 12, max 52): rolling-window size
//
// Empty data degrades cleanly to a 22×22 grid of zero cells.
func (h *Handler) handleArchetypeMatrix(w http.ResponseWriter, r *http.Request) {
	weeks := archetypeMatrixDefaultWeeks
	if v := strings.TrimSpace(r.URL.Query().Get("weeks")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > archetypeMatrixMaxWeeks {
				n = archetypeMatrixMaxWeeks
			}
			weeks = n
		}
	}

	sinceUnix := time.Now().Unix() - int64(weeks)*7*24*3600

	sqlDB := h.cardStatsDB()
	if sqlDB == nil {
		writeJSON(w, heimdall.BuildArchetypeMatrix(nil, heimdall.CanonicalArchetypes))
		return
	}

	groups, err := db.LoadGameSeatGroups(r.Context(), sqlDB, sinceUnix, archetypeMatrixGameCap)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "archetype matrix query failed")
		return
	}

	lookup := h.deckArchetypeLookup()
	games := make([]heimdall.MatrixGameInput, 0, len(groups))
	for _, g := range groups {
		seats := make([]heimdall.MatrixGameSeat, 0, len(g.Seats))
		for _, s := range g.Seats {
			seats = append(seats, heimdall.MatrixGameSeat{
				Archetype: lookup(s.DeckKey),
				Won:       s.Won,
			})
		}
		games = append(games, heimdall.MatrixGameInput{Seats: seats})
	}

	out := heimdall.BuildArchetypeMatrix(games, heimdall.CanonicalArchetypes)
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, out)
}
