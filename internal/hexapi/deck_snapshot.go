package hexapi

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

// handleDeckSnapshot returns a tournament-ready summary for one deck:
// the deck list, commander, Freya bracket + primary archetype, color
// identity, and the deck's recorded W/L stats. Routed at
// GET /api/decks/{owner}/{id}/snapshot.
//
// Purpose: a single round-trip for tournament/seeding tooling that
// needs the FULL identification + matchmaking shape but not the
// per-version DAG, archetype history, or analysis JSON that the
// existing /api/decks/{owner}/{id} + /analysis + /elo-history endpoints
// collectively provide. Stable, side-effect-free, cheap.
//
// Bracket is returned as an integer: 1..5 when known (filename `_bN_`
// marker, falling back to Freya strategy.json), 0 when unresolvable.
// Returning numeric (vs the "?" string the rich-deck endpoint emits)
// matches the matchmaking use case — pairing logic does ordered
// comparisons, not equality checks against sentinel strings.
//
// Record carries the running totals from showmatch_elo (games / wins /
// losses / win_rate / rating). Win rate is a float 0..1 to avoid the
// "percent vs ratio" coin-flip that bites the share-card path
// (share_route.go:101 multiplies by 100). Empty record when no DB,
// no rows, or the showmatch wiring is absent — Wins/Losses/Games are
// zero, win_rate is omitted via omitempty.
func (h *Handler) handleDeckSnapshot(w http.ResponseWriter, r *http.Request) {
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

	commander, bracketStr, color, cmdrCard := resolveDeckMetadata(h.DecksDir, owner, id, deckPath)
	if cmdrCard != "" {
		commander = cmdrCard
	}

	bracket := 0
	if bracketStr != "" && bracketStr != "?" {
		if n, perr := strconv.Atoi(bracketStr); perr == nil {
			bracket = n
		}
	}

	totalCards := 0
	for _, c := range cards {
		if q, ok := c["quantity"].(int); ok {
			totalCards += q
		} else {
			totalCards++
		}
	}

	archetype := loadFreyaPrimaryArchetype(h.DecksDir, owner, id)

	deckKey := owner + "/" + id
	record := loadDeckRecord(r, h, deckKey)

	writeJSON(w, map[string]any{
		"owner":             owner,
		"id":                id,
		"deck_key":          deckKey,
		"commander":         commander,
		"color":             color,
		"bracket":           bracket,
		"primary_archetype": archetype,
		"card_count":        totalCards,
		"cards":             cards,
		"record":            record,
	})
}

// deckRecord is the rolling W/L summary for one deck, projected from
// showmatch_elo. Zero values when the DB has no row for the deck key
// or the showmatch wiring is absent — clients treat games==0 as "no
// history yet" rather than "0 wins, 0 losses".
type deckRecord struct {
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	Losses  int     `json:"losses"`
	WinRate float64 `json:"win_rate,omitempty"`
	Rating  float64 `json:"rating,omitempty"`
}

func loadDeckRecord(r *http.Request, h *Handler, deckKey string) deckRecord {
	rec := deckRecord{}
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		return rec
	}
	var games, wins, losses int
	var rating float64
	err := h.Showmatch.sqlDB.QueryRowContext(r.Context(),
		`SELECT games, wins, losses, rating FROM showmatch_elo WHERE deck_key = ? LIMIT 1`,
		deckKey).Scan(&games, &wins, &losses, &rating)
	if err != nil {
		return rec
	}
	rec.Games = games
	rec.Wins = wins
	rec.Losses = losses
	rec.Rating = rating
	if games > 0 {
		rec.WinRate = float64(wins) / float64(games)
	}
	return rec
}
