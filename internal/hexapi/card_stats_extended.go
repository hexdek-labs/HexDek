package hexapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// handleCardStatsOverview answers GET /api/card-stats/{cardName} with a
// combined view of per-card analytics: overall win rate (from card_stats),
// inclusion rate, top commanders using the card, and bracket distribution.
//
// Response shape:
//
//	{
//	  "card_name":            "Cyclonic Rift",
//	  "win_rate":             0.412,
//	  "games_played":         1234,
//	  "wins":                 508,
//	  "losses":               726,
//	  "inclusion_rate":       0.34,
//	  "decks_using":          17,
//	  "total_decks":          50,
//	  "top_commanders": [
//	    {"commander":"Muldrotha, the Gravetide","games":80,"wins":35,"win_rate":0.4375}
//	  ],
//	  "bracket_distribution": [
//	    {"bracket":3,"count":8},
//	    {"bracket":4,"count":5}
//	  ]
//	}
func (h *Handler) handleCardStatsOverview(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("cardName")
	cardName, err := url.PathUnescape(rawName)
	if err != nil || cardName == "" {
		writeError(w, http.StatusBadRequest, "missing card name")
		return
	}
	cardName = strings.TrimSpace(cardName)

	sqlDB := h.cardStatsDB()

	// Overall stats from card_stats table.
	stat := db.CardStat{CardName: cardName}
	if sqlDB != nil {
		if loaded, lerr := db.LoadCardStat(r.Context(), sqlDB, cardName); lerr == nil {
			stat = loaded
		}
	}

	winRate := 0.0
	if stat.Games > 0 {
		winRate = float64(stat.Wins) / float64(stat.Games)
	}

	// Inclusion rate.
	decksUsing, totalDecks, _ := db.LoadCardInclusionRate(r.Context(), sqlDB, cardName)
	inclusionRate := 0.0
	if totalDecks > 0 {
		inclusionRate = float64(decksUsing) / float64(totalDecks)
	}

	// Top commanders using this card.
	topCommanders, _ := db.LoadCardStatsByCommander(r.Context(), sqlDB, cardName, 10)
	if topCommanders == nil {
		topCommanders = []db.CardCommanderStat{}
	}

	// Bracket distribution.
	bracketDist, _ := db.LoadCardBracketDistribution(r.Context(), sqlDB, cardName)
	if bracketDist == nil {
		bracketDist = []db.CardBracketEntry{}
	}

	resp := db.CardStatsOverview{
		CardName:      stat.CardName,
		WinRate:       winRate,
		GamesPlayed:   stat.Games,
		Wins:          stat.Wins,
		Losses:        stat.Losses,
		InclusionRate: inclusionRate,
		DecksUsing:    decksUsing,
		TotalDecks:    totalDecks,
		TopCommanders: topCommanders,
		BracketDist:   bracketDist,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleCardStatsByCommander answers GET /api/card-stats/{cardName}/by-commander
// with the full per-commander breakdown for a given card.
//
// Response shape:
//
//	{
//	  "card_name":   "Sol Ring",
//	  "commanders": [
//	    {"commander":"Korvold, Fae-Cursed King","games":120,"wins":55,"win_rate":0.458},
//	    ...
//	  ]
//	}
func (h *Handler) handleCardStatsByCommander(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("cardName")
	cardName, err := url.PathUnescape(rawName)
	if err != nil || cardName == "" {
		writeError(w, http.StatusBadRequest, "missing card name")
		return
	}
	cardName = strings.TrimSpace(cardName)

	sqlDB := h.cardStatsDB()
	commanders, _ := db.LoadCardStatsByCommander(r.Context(), sqlDB, cardName, 50)
	if commanders == nil {
		commanders = []db.CardCommanderStat{}
	}

	resp := map[string]any{
		"card_name":   cardName,
		"commanders":  commanders,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = json.NewEncoder(w).Encode(resp)
}
