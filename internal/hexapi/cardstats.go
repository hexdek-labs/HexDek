package hexapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/db"
)

// handleCardStats answers GET /api/cards/{name}/stats with the
// cross-commander aggregate from the card_stats table plus a sample
// of decks currently using the card. Powers the per-card analytics
// section on the Card Page.
//
// Response shape:
//
//	{
//	  "card_name":        "Cyclonic Rift",
//	  "win_rate":         0.412,           // 0..1, 0 when no games yet
//	  "games_played":     123,
//	  "wins":             51,
//	  "losses":           72,
//	  "avg_turn_played":  0,               // reserved; 0 = not measured
//	  "top_decks_using": [{"owner":"josh","id":"breya","commander":"BREYA"}]
//	}
//
// Cards with no recorded games still get a 200 with games_played=0 so
// the frontend can render an honest "no data yet" state.
func (h *Handler) handleCardStats(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("name")
	cardName, err := url.PathUnescape(rawName)
	if err != nil || cardName == "" {
		writeError(w, http.StatusBadRequest, "missing card name")
		return
	}
	cardName = strings.TrimSpace(cardName)

	// Aggregate row may live on the showmatch DB. Fall back to the
	// generic Handler.db if that's how the server was wired.
	stat := db.CardStat{CardName: cardName}
	if sqlDB := h.cardStatsDB(); sqlDB != nil {
		if loaded, lerr := db.LoadCardStat(r.Context(), sqlDB, cardName); lerr == nil {
			stat = loaded
		}
	}

	winRate := 0.0
	if stat.Games > 0 {
		winRate = float64(stat.Wins) / float64(stat.Games)
	}

	resp := map[string]any{
		"card_name":       stat.CardName,
		"games_played":    stat.Games,
		"wins":            stat.Wins,
		"losses":          stat.Losses,
		"win_rate":        winRate,
		"avg_turn_played": stat.AvgTurnPlayed,
		"top_decks_using": h.topDecksUsing(cardName, 5),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=60")
	_ = json.NewEncoder(w).Encode(resp)
}

// cardStatsDB picks the right *sql.DB for card_stats lookups. The
// showmatch handler holds the canonical DB used by the persistGame
// recorder; the generic Handler.db is a fallback for setups where
// SetDB was called but no Showmatch was wired.
func (h *Handler) cardStatsDB() *sql.DB {
	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		return h.Showmatch.sqlDB
	}
	return h.db
}

type deckMatch struct {
	Owner     string `json:"owner"`
	ID        string `json:"id"`
	Commander string `json:"commander"`
}

// topDecksUsing scans h.DecksDir for decks containing cardName, returns
// up to limit entries (newest-modified first). This is a directory walk,
// not a SQL query — we don't have a reverse index from card → decks. At
// the current scale (~hundreds of decks) the walk is fine; if the deck
// pool grows much larger this should move to a precomputed index.
func (h *Handler) topDecksUsing(cardName string, limit int) []deckMatch {
	if h.DecksDir == "" || cardName == "" {
		return []deckMatch{}
	}
	needle := strings.ToLower(cardName)
	type candidate struct {
		match deckMatch
		mtime int64
	}
	var found []candidate

	owners, err := os.ReadDir(h.DecksDir)
	if err != nil {
		return []deckMatch{}
	}
	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		if owner == "freya" || owner == "benched" || owner == "test" || owner == "moxfield_300" || owner == ".versions" {
			continue
		}
		ownerDir := filepath.Join(h.DecksDir, owner)
		files, ferr := os.ReadDir(ownerDir)
		if ferr != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			n := f.Name()
			if !strings.HasSuffix(n, ".txt") && !strings.HasSuffix(n, ".json") {
				continue
			}
			full := filepath.Join(ownerDir, n)
			if !deckContainsCard(full, needle) {
				continue
			}
			id := strings.TrimSuffix(n, filepath.Ext(n))
			commander, _, _ := parseDeckFilename(id)
			cm := deckMatch{Owner: owner, ID: id, Commander: commander}
			info, _ := f.Info()
			var mt int64
			if info != nil {
				mt = info.ModTime().Unix()
			}
			found = append(found, candidate{match: cm, mtime: mt})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].mtime > found[j].mtime })
	if limit <= 0 {
		limit = 5
	}
	if len(found) > limit {
		found = found[:limit]
	}
	out := make([]deckMatch, 0, len(found))
	for _, c := range found {
		out = append(out, c.match)
	}
	return out
}
