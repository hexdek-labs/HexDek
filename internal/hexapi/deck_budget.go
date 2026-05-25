package hexapi

import (
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/hexdek/hexdek/internal/oracle"
)

// handleDeckBudget answers GET /api/decks/{owner}/{id}/budget with a
// per-card price rollup pulled from the Scryfall-backed oracle cache.
//
// Response shape (all amounts are dollars-and-cents floats; missing
// data renders as 0.00 with `priced=false` on the affected row so
// the frontend can flag "no market price" cards without losing the
// rest of the total):
//
//	{
//	  "currency":      "USD",
//	  "total":         148.32,
//	  "card_count":    99,
//	  "unique_count":  82,
//	  "priced_count":  79,    // unique cards we could price
//	  "missing_count": 3,     // unique cards Scryfall has no $ for
//	  "cards": [
//	    {"name":"Mana Crypt","qty":1,"unit":89.99,"line":89.99,"priced":true},
//	    ...                   // sorted by line desc, then name asc
//	  ],
//	  "top_expensive": [...]  // first 5 of cards by line value (priced rows only)
//	}
//
// Basic lands (Plains/Island/Swamp/Mountain/Forest + snow variants +
// Wastes) skip the lookup — every printing is functionally $0.10 and
// the noise drowns out the rest of the list.
func (h *Handler) handleDeckBudget(w http.ResponseWriter, r *http.Request) {
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
	if len(cards) == 0 {
		writeJSON(w, deckBudgetEmpty())
		return
	}

	// Build per-name qty map (dedupes the rare case where a deck list
	// contains the same card on two lines — e.g., a basic land split
	// across two sections).
	qtyByName := map[string]int{}
	displayName := map[string]string{}
	totalCards := 0
	for _, c := range cards {
		name, _ := c["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		qty := 1
		if q, ok := c["quantity"].(int); ok && q > 0 {
			qty = q
		}
		key := strings.ToLower(name)
		qtyByName[key] += qty
		if _, seen := displayName[key]; !seen {
			displayName[key] = name
		}
		totalCards += qty
	}

	// Resolve prices for the non-basic subset only. Basics are flagged
	// as priced=true with unit=0 so they appear in the per-card list
	// without distorting the total.
	uniqueNames := make([]string, 0, len(qtyByName))
	for key := range qtyByName {
		if !budgetIsBasicLand(key) {
			uniqueNames = append(uniqueNames, displayName[key])
		}
	}

	var resolved map[string]*oracle.Card
	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		resolved = oracle.RefreshPrices(r.Context(), h.Showmatch.sqlDB, uniqueNames)
	}

	type row struct {
		Name   string  `json:"name"`
		Qty    int     `json:"qty"`
		Unit   float64 `json:"unit"`
		Line   float64 `json:"line"`
		Priced bool    `json:"priced"`
	}
	rows := make([]row, 0, len(qtyByName))
	total := 0.0
	pricedUnique := 0
	missingUnique := 0
	for key, qty := range qtyByName {
		display := displayName[key]
		if budgetIsBasicLand(key) {
			rows = append(rows, row{Name: display, Qty: qty, Unit: 0, Line: 0, Priced: true})
			pricedUnique++
			continue
		}
		card := resolved[key]
		unit, ok := budgetUnitPrice(card)
		if !ok {
			rows = append(rows, row{Name: display, Qty: qty, Unit: 0, Line: 0, Priced: false})
			missingUnique++
			continue
		}
		line := unit * float64(qty)
		rows = append(rows, row{Name: display, Qty: qty, Unit: unit, Line: line, Priced: true})
		total += line
		pricedUnique++
	}

	// Sort rows by line value desc, then name asc — most expensive
	// first so the budget panel can show the top contributors without
	// re-sorting client-side.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Line != rows[j].Line {
			return rows[i].Line > rows[j].Line
		}
		return strings.ToLower(rows[i].Name) < strings.ToLower(rows[j].Name)
	})

	// top_expensive — first 5 priced rows (basics with unit=0 sink
	// naturally via the line sort, but filter explicitly so a deck
	// with 99 basics still renders top-N from the spell list).
	topExpensive := []row{}
	for _, r := range rows {
		if !r.Priced || r.Line == 0 {
			continue
		}
		topExpensive = append(topExpensive, r)
		if len(topExpensive) >= 5 {
			break
		}
	}

	writeJSON(w, map[string]any{
		"currency":      "USD",
		"total":         round2(total),
		"card_count":    totalCards,
		"unique_count":  len(qtyByName),
		"priced_count":  pricedUnique,
		"missing_count": missingUnique,
		"cards":         rows,
		"top_expensive": topExpensive,
	})
}

// deckBudgetEmpty is the response shape for an empty deck — all zeros
// with empty lists, so the frontend renders a $0 panel rather than a
// "loading…" stuck state.
func deckBudgetEmpty() map[string]any {
	return map[string]any{
		"currency":      "USD",
		"total":         0.0,
		"card_count":    0,
		"unique_count":  0,
		"priced_count":  0,
		"missing_count": 0,
		"cards":         []any{},
		"top_expensive": []any{},
	}
}

// budgetUnitPrice extracts the canonical USD market price from a
// resolved oracle.Card. Falls back to usd_etched and usd_foil in that
// order — most cards have plain `usd` populated, but some borderless /
// promo printings only ship etched or foil pricing. Returns ok=false
// when no usable price is present so the caller can flag the row.
func budgetUnitPrice(card *oracle.Card) (float64, bool) {
	if card == nil || len(card.Prices) == 0 {
		return 0, false
	}
	for _, key := range []string{"usd", "usd_etched", "usd_foil"} {
		raw := strings.TrimSpace(card.Prices[key])
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err == nil && v > 0 {
			return v, true
		}
	}
	return 0, false
}

// budgetIsBasicLand recognises the names parseDeckList emits for
// printing-agnostic basics. Mirrors the BASIC_LANDS set in
// hexdek/src/lib/deckParser.js so frontend + backend agree on what
// "free" cards look like.
func budgetIsBasicLand(lowerName string) bool {
	switch lowerName {
	case "plains", "island", "swamp", "mountain", "forest",
		"snow-covered plains", "snow-covered island", "snow-covered swamp",
		"snow-covered mountain", "snow-covered forest",
		"wastes":
		return true
	}
	return false
}

// round2 returns v rounded to 2 decimal places. Avoids floating-point
// noise like $148.31999999999996 in the JSON response.
func round2(v float64) float64 {
	// Add 0.5 in the smallest place and truncate via int cast — same
	// shape as math.Round, no dep on math.
	if v >= 0 {
		return float64(int(v*100+0.5)) / 100
	}
	return float64(int(v*100-0.5)) / 100
}
