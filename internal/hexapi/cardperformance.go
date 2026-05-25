package hexapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// handleCardPerformance answers GET /api/cards/{name}/performance with
// the per-card "did this card actually do anything" aggregate from the
// card_performance table. Distinct from /api/cards/{name}/stats: stats
// counts every game the card was in any deck list, performance counts
// only games where a seat actually drew the card and held it in some
// non-library/non-command zone at game end.
//
// Response shape:
//
//	{
//	  "card_name":             "Sol Ring",
//	  "games_included":        912,
//	  "wins_when_included":    241,
//	  "win_rate":              0.264,           // 0..1; 0 when games_included == 0
//	  "avg_turn_played":       1.4,             // 0 when no turn data captured
//	  "avg_battlefield_time":  6.8              // 0 when no observation captured
//	}
//
// Cards with no recorded plays still get 200 with zero counts so the
// frontend can render an honest "no data yet" state.
func (h *Handler) handleCardPerformance(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("name")
	cardName, err := url.PathUnescape(rawName)
	if err != nil || cardName == "" {
		writeError(w, http.StatusBadRequest, "missing card name")
		return
	}
	cardName = strings.TrimSpace(cardName)

	perf := db.CardPerformance{CardName: cardName}
	if sqlDB := h.cardStatsDB(); sqlDB != nil {
		if loaded, lerr := db.LoadCardPerformance(r.Context(), sqlDB, cardName); lerr == nil {
			perf = loaded
		}
	}

	winRate := 0.0
	if perf.GamesIncluded > 0 {
		winRate = float64(perf.WinsWhenIncluded) / float64(perf.GamesIncluded)
	}

	resp := map[string]any{
		"card_name":            perf.CardName,
		"games_included":       perf.GamesIncluded,
		"wins_when_included":   perf.WinsWhenIncluded,
		"win_rate":             winRate,
		"avg_turn_played":      perf.AvgTurnPlayed,
		"avg_battlefield_time": perf.AvgBattlefieldTime,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = json.NewEncoder(w).Encode(resp)
}
