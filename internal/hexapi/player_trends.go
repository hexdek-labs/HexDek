package hexapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/heimdall"
)

const (
	// playerTrendsDefaultWindow is what /api/players/{id}/trends uses
	// when ?window is absent. 30 matches the "rolling 30-game" framing
	// in the dashboard surface; larger windows are allowed up to the
	// max to keep one DB scan reasonable.
	playerTrendsDefaultWindow = 30
	// playerTrendsMaxWindow caps how many games one trends call can
	// scan. 200 keeps the per-game opponents subquery in
	// db.LoadOwnerGames bounded (200 × 1 = 200 extra round-trips
	// worst case) while still letting power users see a season's
	// worth of play.
	playerTrendsMaxWindow = 200
)

// handlePlayerTrends implements GET /api/players/{id}/trends.
//
// Query params:
//   - window (optional, default 30, max 200): rolling window size
//
// Returns 200 + PlayerTrends body. When the player has no recorded
// games, returns a well-formed payload with GamesPlayed=0 and empty
// breakdowns — the frontend can render an empty state without
// distinguishing missing-player from never-played.
func (h *Handler) handlePlayerTrends(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid player id")
		return
	}
	player := strings.ToLower(id)

	window := playerTrendsDefaultWindow
	if v := strings.TrimSpace(r.URL.Query().Get("window")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > playerTrendsMaxWindow {
				n = playerTrendsMaxWindow
			}
			window = n
		}
	}

	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, heimdall.ComputePlayerTrends(player, window, nil, nil))
		return
	}

	rows, err := db.LoadOwnerGames(r.Context(), h.Showmatch.sqlDB, player, window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "trends query failed")
		return
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

	trends := heimdall.ComputePlayerTrends(player, window, games, h.deckArchetypeLookup())
	writeJSON(w, trends)
}

// deckArchetypeLookup returns a memoizing lookup function suitable
// for passing into heimdall.ComputePlayerTrends. The closure reads
// `<DecksDir>/<owner>/freya/<id>.strategy.json` once per unique
// deck_key per call — repeated games against the same deck don't
// re-stat the file.
//
// Returns the empty string when the strategy file is missing or
// archetype is unset; heimdall translates that into the "unknown"
// bucket.
func (h *Handler) deckArchetypeLookup() func(deckKey string) string {
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
		strategyFile := filepath.Join(h.DecksDir, parts[0], "freya", parts[1]+".strategy.json")
		data, err := os.ReadFile(strategyFile)
		if err != nil {
			cache[deckKey] = ""
			return ""
		}
		var strat struct {
			Archetype string `json:"archetype"`
		}
		if err := json.Unmarshal(data, &strat); err != nil {
			cache[deckKey] = ""
			return ""
		}
		slug := strings.TrimSpace(strat.Archetype)
		cache[deckKey] = slug
		return slug
	}
}
