package hexapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

// metaDeckArchetypeLookup is the same pattern as
// Handler.deckArchetypeLookup but standalone — kept for tests that
// inject a lookup directly without spinning a full Handler.
func metaDeckArchetypeLookup(decksDir string) func(deckKey string) string {
	cache := make(map[string]string)
	return func(deckKey string) string {
		if deckKey == "" {
			return ""
		}
		if v, ok := cache[deckKey]; ok {
			return v
		}
		parts := strings.SplitN(deckKey, "/", 2)
		if len(parts) != 2 {
			cache[deckKey] = ""
			return ""
		}
		strategyFile := filepath.Join(decksDir, parts[0], "freya", parts[1]+".strategy.json")
		data, err := os.ReadFile(strategyFile)
		if err != nil {
			cache[deckKey] = ""
			return ""
		}
		var strat struct {
			Archetype string `json:"archetype"`
		}
		if json.Unmarshal(data, &strat) != nil {
			cache[deckKey] = ""
			return ""
		}
		slug := strings.TrimSpace(strat.Archetype)
		cache[deckKey] = slug
		return slug
	}
}
