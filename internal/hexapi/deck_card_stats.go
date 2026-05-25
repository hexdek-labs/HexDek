package hexapi

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"sort"
	"strings"
)

// Per-deck Hot Cards endpoint.
//
// The Hot Cards widget currently ranks cards using commander-level
// stats (handleCardWinStats). That signal is shared by every deck for
// a given commander, so two Breya pilots see the same list even if
// their builds barely overlap. This endpoint intersects card_stats
// (cross-commander, deck-list-participation aggregate) with the actual
// card list of one deck and ranks the entries by win-rate-above-
// baseline — a richer per-deck-aware signal.

// Tunables. Conservative so the widget doesn't surface noise.
const (
	deckCardStatsMinGames    = 5  // a card needs this many recorded games before it can rank
	deckCardStatsMaxResults  = 25
	deckCardStatsBaselineMin = 50 // min total games across the pool before we trust the data-driven baseline
	// 4-player Commander's a-priori per-seat win rate. Used as the
	// baseline when card_stats is too sparse to estimate one.
	deckCardStatsFallbackBaseline = 0.25
)

// DeckCardStatEntry is one ranked card in the response.
type DeckCardStatEntry struct {
	CardName     string  `json:"card_name"`
	Games        int     `json:"games"`
	Wins         int     `json:"wins"`
	Losses       int     `json:"losses"`
	WinRate      float64 `json:"win_rate"`        // 0..1
	WinRateDelta float64 `json:"win_rate_delta"`  // win_rate - baseline
}

// handleDeckCardStats answers GET /api/deck-card-stats/{owner}/{id}.
//
// Response shape:
//
//	{
//	  "owner":             "josh",
//	  "id":                "breya",
//	  "commander":         "Breya, Etherium Shaper",
//	  "baseline_win_rate": 0.25,
//	  "min_games":         5,
//	  "deck_size":         100,
//	  "matched_cards":     42,
//	  "cards": [
//	    {
//	      "card_name":      "Sol Ring",
//	      "games":          120,
//	      "wins":           35,
//	      "losses":         85,
//	      "win_rate":       0.2917,
//	      "win_rate_delta": 0.0417
//	    },
//	    ...
//	  ]
//	}
//
// A deck whose cards have no card_stats coverage still gets a 200 with
// an empty cards list so the widget can render a "no data yet" state.
func (h *Handler) handleDeckCardStats(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}
	data, err := os.ReadFile(deckPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read deck")
		return
	}
	var cards []map[string]any
	if strings.HasSuffix(deckPath, ".json") {
		cards = parseDeckJSON(data)
	} else {
		cards = parseDeckList(string(data))
	}

	commanderName := strings.TrimSpace(extractCommander(deckPath))
	commanderLower := strings.ToLower(commanderName)

	sqlDB := h.cardStatsDB()
	statsLower, baseline, err := loadDeckCardStatsPool(r.Context(), sqlDB)
	if err != nil {
		// Surface as empty rather than 500 — the widget should still
		// render the deck list with a "no data yet" affordance.
		writeJSON(w, deckCardStatsResponse(owner, id, commanderName, baseline, len(cards), 0, nil))
		return
	}

	// Dedup by lowercase card name so a deck listing a card twice (sideboard
	// quirks, partner setups) doesn't show it twice in the response.
	seen := make(map[string]bool, len(cards))
	entries := make([]DeckCardStatEntry, 0, len(cards))
	for _, c := range cards {
		name, _ := c["name"].(string)
		if name == "" {
			continue
		}
		base := stripSetCode(name)
		key := strings.ToLower(base)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if commanderLower != "" && key == commanderLower {
			continue
		}
		st, ok := statsLower[key]
		if !ok || st.Games < deckCardStatsMinGames {
			continue
		}
		wr := float64(st.Wins) / float64(st.Games)
		entries = append(entries, DeckCardStatEntry{
			CardName:     st.Name,
			Games:        st.Games,
			Wins:         st.Wins,
			Losses:       st.Games - st.Wins,
			WinRate:      wr,
			WinRateDelta: wr - baseline,
		})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].WinRateDelta != entries[j].WinRateDelta {
			return entries[i].WinRateDelta > entries[j].WinRateDelta
		}
		if entries[i].Games != entries[j].Games {
			return entries[i].Games > entries[j].Games
		}
		return entries[i].CardName < entries[j].CardName
	})
	matched := len(entries)
	if len(entries) > deckCardStatsMaxResults {
		entries = entries[:deckCardStatsMaxResults]
	}

	writeJSON(w, deckCardStatsResponse(owner, id, commanderName, baseline, len(cards), matched, entries))
}

func deckCardStatsResponse(owner, id, commander string, baseline float64, deckSize, matched int, entries []DeckCardStatEntry) map[string]any {
	if entries == nil {
		entries = []DeckCardStatEntry{}
	}
	return map[string]any{
		"owner":             owner,
		"id":                id,
		"commander":         commander,
		"baseline_win_rate": baseline,
		"min_games":         deckCardStatsMinGames,
		"deck_size":         deckSize,
		"matched_cards":     matched,
		"cards":             entries,
	}
}

// loadDeckCardStatsPool pulls every card_stats row in one query and
// derives the baseline win rate from the same pool. Using a single
// shared scan keeps the per-request cost O(rows in card_stats) rather
// than O(deck_size * card_stats_rows).
//
// Baseline is the games-weighted mean win rate across cards with
// >= deckCardStatsMinGames recorded games — i.e. wins/games summed
// over the qualifying pool. If the pool is too thin to be meaningful
// (< deckCardStatsBaselineMin games), we fall back to the a-priori
// 25% per-seat win rate for 4-player Commander rather than letting a
// handful of biased rows define the comparison line.
func loadDeckCardStatsPool(ctx context.Context, sqlDB *sql.DB) (map[string]deckCardStatRow, float64, error) {
	if sqlDB == nil {
		return map[string]deckCardStatRow{}, deckCardStatsFallbackBaseline, nil
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT card_name, wins, games FROM card_stats WHERE games > 0`)
	if err != nil {
		return nil, deckCardStatsFallbackBaseline, err
	}
	defer rows.Close()
	out := make(map[string]deckCardStatRow, 4096)
	var totalWins, totalGames int
	for rows.Next() {
		var r deckCardStatRow
		if err := rows.Scan(&r.Name, &r.Wins, &r.Games); err != nil {
			return nil, deckCardStatsFallbackBaseline, err
		}
		out[strings.ToLower(r.Name)] = r
		if r.Games >= deckCardStatsMinGames {
			totalWins += r.Wins
			totalGames += r.Games
		}
	}
	if err := rows.Err(); err != nil {
		return nil, deckCardStatsFallbackBaseline, err
	}
	baseline := deckCardStatsFallbackBaseline
	if totalGames >= deckCardStatsBaselineMin {
		baseline = float64(totalWins) / float64(totalGames)
	}
	return out, baseline, nil
}

// deckCardStatRow is the in-handler subset of card_stats we need for
// ranking. Kept local rather than reusing upgrade.go's type so the two
// endpoints' tuning constants can evolve independently.
type deckCardStatRow struct {
	Name  string
	Wins  int
	Games int
}
