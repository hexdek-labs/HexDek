package hexapi

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"sort"
	"strings"
)

// upgradeCardStat is the in-handler subset of card_stats we need for
// suggestion ranking. Kept local so this endpoint doesn't take a hard
// dependency on the db package's CardStat shape.
type upgradeCardStat struct {
	Name  string
	Wins  int
	Games int
}

// loadUpgradeCardStats pulls every card_stats row with games >= minGames
// in one query. Returns a map keyed by lowercase card name for matching
// against deck/oracle data which is also lowercase-indexed.
func loadUpgradeCardStats(ctx context.Context, sqlDB *sql.DB, minGames int) (map[string]upgradeCardStat, error) {
	if sqlDB == nil {
		return map[string]upgradeCardStat{}, nil
	}
	rows, err := sqlDB.QueryContext(ctx,
		`SELECT card_name, wins, games FROM card_stats WHERE games >= ?`, minGames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]upgradeCardStat, 4096)
	for rows.Next() {
		var s upgradeCardStat
		if err := rows.Scan(&s.Name, &s.Wins, &s.Games); err != nil {
			return nil, err
		}
		out[strings.ToLower(s.Name)] = s
	}
	return out, rows.Err()
}

// Tunables. Kept conservative so suggestions are defensible: the
// candidate must have a meaningfully larger sample than the current
// card, and the win-rate edge must clear noise.
const (
	upgradeMinCurrentGames = 10  // current card needs this many games to have a baseline
	upgradeMinCandidateGames = 25 // candidate needs this many games to be considered
	upgradeMinDeltaPct      = 5.0 // candidate must beat current by >= this many percentage points
	upgradeMaxResults       = 10
)

// UpgradeSuggestion is one row of the response. Win rates are 0-100.
type UpgradeSuggestion struct {
	CurrentCard   string  `json:"current_card"`
	SuggestedCard string  `json:"suggested_card"`
	CurrentWinRate   float64 `json:"current_win_rate"`
	SuggestedWinRate float64 `json:"suggested_win_rate"`
	WinRateDelta  float64 `json:"win_rate_delta"`
	Reason        string  `json:"reason"`
}

// handleDeckUpgrade implements GET /api/decks/{owner}/{id}/upgrade.
//
// For each non-land non-commander card in the deck, scans the oracle
// card pool for a strictly-better candidate: same primary type, same
// (or subset) color identity, CMC <= current, NOT already in the deck,
// and a higher cross-commander win rate from card_stats with adequate
// sample size. Returns the top suggestions ranked by win-rate delta.
//
// Response: { suggestions: [...], meta: {...} }. An empty list is a
// valid 200 response when the deck has no card_stats coverage yet.
func (h *Handler) handleDeckUpgrade(w http.ResponseWriter, r *http.Request) {
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
	commanderName := strings.ToLower(strings.TrimSpace(extractCommander(deckPath)))

	if h.cardDB == nil {
		writeJSON(w, map[string]any{
			"suggestions": []UpgradeSuggestion{},
			"meta":        map[string]any{"reason": "card oracle DB not loaded"},
		})
		return
	}

	statsLower, err := loadUpgradeCardStats(r.Context(), h.db, upgradeMinCurrentGames)
	if err != nil || len(statsLower) == 0 {
		writeJSON(w, map[string]any{
			"suggestions": []UpgradeSuggestion{},
			"meta":        map[string]any{"reason": "no card_stats data — play more games"},
		})
		return
	}

	deckSet := make(map[string]bool, len(cards))
	for _, c := range cards {
		if name, ok := c["name"].(string); ok && name != "" {
			deckSet[strings.ToLower(stripSetCode(name))] = true
		}
	}

	// Precompute candidate index: lowercased name -> oracleCard, plus
	// derived primary-type and color set. Filter to non-land cards with
	// any stats coverage so the inner loop only walks viable rows.
	type candidate struct {
		name        string
		cmc         float64
		primaryType string
		colors      string // sorted "BGR" etc.
		winRate     float64
		games       int
	}
	candidates := make([]candidate, 0, 4096)
	for nameLower, oc := range h.cardDB {
		if isLandType(oc.TypeLine) {
			continue
		}
		st, ok := statsLower[nameLower]
		if !ok || st.Games < upgradeMinCandidateGames {
			continue
		}
		candidates = append(candidates, candidate{
			name:        st.Name, // preserves original casing
			cmc:         oc.CMC,
			primaryType: primaryTypeOf(oc.TypeLine),
			colors:      colorsFromManaCost(oc.ManaCost),
			winRate:     winRatePct(st.Wins, st.Games),
			games:       st.Games,
		})
	}

	suggestions := make([]UpgradeSuggestion, 0, upgradeMaxResults*2)
	for _, c := range cards {
		name, _ := c["name"].(string)
		if name == "" {
			continue
		}
		baseName := stripSetCode(name)
		baseLower := strings.ToLower(baseName)
		if commanderName != "" && baseLower == commanderName {
			continue
		}
		oc, ok := h.cardDB[baseLower]
		if !ok || isLandType(oc.TypeLine) {
			continue
		}
		stat, ok := statsLower[baseLower]
		if !ok || stat.Games < upgradeMinCurrentGames {
			continue
		}
		curWR := winRatePct(stat.Wins, stat.Games)
		curType := primaryTypeOf(oc.TypeLine)
		curColors := colorsFromManaCost(oc.ManaCost)

		var best *candidate
		for i := range candidates {
			cand := &candidates[i]
			if cand.cmc > oc.CMC {
				continue
			}
			if cand.primaryType != curType {
				continue
			}
			// Strict color-equivalence: candidate's color set must equal
			// current card's. Equivalent colors avoids accidentally
			// recommending a card outside the deck's color identity and
			// keeps the role (e.g., a {U}{B} removal stays a {U}{B}
			// alternative). Subset-only would over-recommend colorless.
			if cand.colors != curColors {
				continue
			}
			candLower := strings.ToLower(cand.name)
			if deckSet[candLower] || candLower == baseLower {
				continue
			}
			delta := cand.winRate - curWR
			if delta < upgradeMinDeltaPct {
				continue
			}
			if best == nil || cand.winRate > best.winRate {
				cc := *cand
				best = &cc
			}
		}
		if best == nil {
			continue
		}
		suggestions = append(suggestions, UpgradeSuggestion{
			CurrentCard:      baseName,
			SuggestedCard:    best.name,
			CurrentWinRate:   round1(curWR),
			SuggestedWinRate: round1(best.winRate),
			WinRateDelta:     round1(best.winRate - curWR),
			Reason: buildReason(curType, curColors, oc.CMC, best.cmc,
				curWR, best.winRate, stat.Games, best.games),
		})
	}

	sort.SliceStable(suggestions, func(i, j int) bool {
		if suggestions[i].WinRateDelta != suggestions[j].WinRateDelta {
			return suggestions[i].WinRateDelta > suggestions[j].WinRateDelta
		}
		return suggestions[i].CurrentCard < suggestions[j].CurrentCard
	})
	if len(suggestions) > upgradeMaxResults {
		suggestions = suggestions[:upgradeMaxResults]
	}

	writeJSON(w, map[string]any{
		"suggestions": suggestions,
		"meta": map[string]any{
			"deck_size":         len(cards),
			"candidate_pool":    len(candidates),
			"min_current_games": upgradeMinCurrentGames,
			"min_candidate_games": upgradeMinCandidateGames,
			"min_delta_pct":     upgradeMinDeltaPct,
		},
	})
}

func stripSetCode(name string) string {
	if idx := strings.Index(name, "("); idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	return strings.TrimSpace(name)
}

func isLandType(typeLine string) bool {
	return strings.Contains(strings.ToLower(typeLine), "land")
}

// primaryTypeOf collapses a Scryfall type line to one of: creature,
// instant, sorcery, artifact, enchantment, planeswalker, battle, land.
// Returns "other" for unrecognised lines. The check order matters:
// creature wins over artifact for "Artifact Creature" so a Bear-stat
// suggestion competes with the right candidate pool.
func primaryTypeOf(typeLine string) string {
	tl := strings.ToLower(typeLine)
	switch {
	case strings.Contains(tl, "creature"):
		return "creature"
	case strings.Contains(tl, "planeswalker"):
		return "planeswalker"
	case strings.Contains(tl, "battle"):
		return "battle"
	case strings.Contains(tl, "instant"):
		return "instant"
	case strings.Contains(tl, "sorcery"):
		return "sorcery"
	case strings.Contains(tl, "enchantment"):
		return "enchantment"
	case strings.Contains(tl, "artifact"):
		return "artifact"
	case strings.Contains(tl, "land"):
		return "land"
	}
	return "other"
}

func winRatePct(wins, games int) float64 {
	if games <= 0 {
		return 0
	}
	return float64(wins) / float64(games) * 100
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func buildReason(curType, colors string, curCmc, candCmc, curWR, candWR float64, curGames, candGames int) string {
	colorTag := colors
	if colorTag == "" {
		colorTag = "C"
	}
	cmcNote := "same CMC"
	if candCmc < curCmc {
		cmcNote = "lower CMC"
	}
	return joinNonempty([]string{
		formatPct(candWR) + " vs " + formatPct(curWR) + " win rate",
		"+" + formatDelta(candWR-curWR) + "pp",
		cmcNote,
		curType + " · " + colorTag,
		formatGames(candGames) + " games",
	}, " · ")
}

func joinNonempty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

func formatPct(v float64) string {
	return formatFloat1(v) + "%"
}

func formatDelta(v float64) string {
	if v < 0 {
		return formatFloat1(v)
	}
	return formatFloat1(v)
}

func formatFloat1(v float64) string {
	whole := int(v)
	frac := int(v*10+0.5) - whole*10
	if frac < 0 {
		frac = -frac
	}
	if v < 0 && whole == 0 {
		return "-0." + itoa(frac)
	}
	return itoa(whole) + "." + itoa(frac)
}

func formatGames(n int) string { return itoa(n) }
