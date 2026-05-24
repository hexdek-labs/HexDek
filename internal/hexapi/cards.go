package hexapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hexdek/hexdek/internal/cardstats"
)

// CardSearchHit is a row in the /api/cards/search payload.
type CardSearchHit struct {
	Name     string  `json:"name"`
	ManaCost string  `json:"mana_cost"`
	TypeLine string  `json:"type_line"`
	CMC      float64 `json:"cmc"`
	Set      string  `json:"set,omitempty"`
	ImageURI string  `json:"image_uri"`
	Score    int     `json:"score,omitempty"`
}

// CardDetail is the response for /api/cards/{name}.
type CardDetail struct {
	Name       string  `json:"name"`
	ManaCost   string  `json:"mana_cost"`
	TypeLine   string  `json:"type_line"`
	OracleText string  `json:"oracle_text"`
	CMC        float64 `json:"cmc"`
	Set        string  `json:"set,omitempty"`
	ImageURI   string  `json:"image_uri"`

	// Printings is empty when the loaded corpus is the oracle-cards bulk
	// (one entry per name). Switch the loader to default-cards.json to
	// populate this with per-set printings.
	Printings []CardPrinting `json:"printings"`

	// DecksUsing lists every loaded deck that runs this card, with the
	// deck's Freya role assignment when available.
	DecksUsing []DeckRef `json:"decks_using"`

	// DeckCount is the total number of loaded decks containing this card.
	// (Equal to len(DecksUsing) but surfaced as a top-level number so
	// the frontend doesn't need to count when DecksUsing is paginated
	// or otherwise capped client-side.)
	DeckCount int `json:"deck_count"`

	// SynergyPartners is the top 10 cards that most frequently appear
	// alongside this one across loaded decks. Sorted by Count desc.
	SynergyPartners []SynergyPartner `json:"synergy_partners"`

	// RoleDistribution counts how often this card is tagged with each
	// Freya role across decks that have an analyzed card_roles map.
	// Keys are role strings (ramp/draw/removal/combo/threat/utility/...);
	// missing role assignments are tallied under "unassigned".
	RoleDistribution map[string]int `json:"role_distribution"`

	// AvgDeckBracket is the mean Freya bracket (1-5) of decks running
	// this card, computed only over decks with a non-zero bracket in
	// strategy.json. 0 when no analyzed decks contain the card.
	AvgDeckBracket float64 `json:"avg_deck_bracket"`
}

type CardPrinting struct {
	Set      string `json:"set"`
	SetName  string `json:"set_name,omitempty"`
	Released string `json:"released_at,omitempty"`
	ImageURI string `json:"image_uri,omitempty"`
}

type DeckRef struct {
	Owner     string `json:"owner"`
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Commander string `json:"commander,omitempty"`
	// Role is the Freya card-role tag (ramp/draw/removal/combo/...) if a
	// strategy.json with card_roles exists for this deck. Empty string
	// means the deck has not been analyzed or the role is unset.
	Role string `json:"role,omitempty"`
}

type SynergyPartner struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	// Decks samples up to ~5 deck names where the partner co-occurs;
	// keeps the payload small while letting the UI show provenance.
	Decks []string `json:"decks,omitempty"`
}

// strategyMeta caches the fields we read from a deck's strategy.json.
type strategyMeta struct {
	CardRoles map[string]string `json:"card_roles"`
	Bracket   int               `json:"bracket"`
}

// cardImageURI returns the relative URL the frontend uses to fetch art
// for a card. It mirrors the existing /api/card-art/{name} contract,
// which 302s to Scryfall's named-fuzzy art_crop image. Frontend code
// can prepend the API base or rely on same-origin deployment.
func cardImageURI(name string) string {
	return "/api/card-art/" + url.PathEscape(name)
}

// handleCardSearch implements GET /api/cards/search?q=<query>&limit=<n>.
// Searches the loaded oracle card corpus by name with prefix/substring
// fuzzy match, returning the top results sorted by relevance.
//
// Defaults: limit=20, max=50. Empty `q` returns an empty list.
func (h *Handler) handleCardSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(h.cardDB) == 0 {
		writeJSON(w, map[string]any{"query": q, "results": []CardSearchHit{}})
		return
	}
	needle := strings.ToLower(q)

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n := parseInt(v); n > 0 {
			limit = n
		}
	}
	if limit > 50 {
		limit = 50
	}

	hits := make([]CardSearchHit, 0, 64)
	for nameLower, info := range h.cardDB {
		score := matchScore(nameLower, needle)
		if score == 0 {
			continue
		}
		display := titleCaseCardName(nameLower)
		hits = append(hits, CardSearchHit{
			Name:     display,
			ManaCost: info.ManaCost,
			TypeLine: info.TypeLine,
			CMC:      info.CMC,
			Set:      info.Set,
			ImageURI: cardImageURI(display),
			Score:    score,
		})
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Name < hits[j].Name
	})

	if len(hits) > limit {
		hits = hits[:limit]
	}

	writeJSON(w, map[string]any{
		"query":   q,
		"count":   len(hits),
		"results": hits,
	})
}

// handleCardAnalytics implements GET /api/analytics/cards.
//
// Dumps the top 50 highest- and lowest-win-rate cards across the
// in-memory cardstats store (populated by the grinder's game-end
// hook). Min-game threshold defaults to 10 and is overridable via
// ?min=N. The "limit" query param caps each side; defaults to 50.
//
// Empty payload until the grinder has finished enough games.
func (h *Handler) handleCardAnalytics(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n := parseInt(v); n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	minGames := 10
	if v := r.URL.Query().Get("min"); v != "" {
		if n := parseInt(v); n > 0 {
			minGames = n
		}
	}

	top, bottom := cardstats.Default.TopBottom(limit, minGames)
	writeJSON(w, map[string]any{
		"min_games":     minGames,
		"limit":         limit,
		"unique_cards":  cardstats.Default.Size(),
		"top":           top,
		"bottom":        bottom,
	})
}

// handleCardByName implements GET /api/cards/{name}.
//
// Returns full card data plus deck cross-reference and synergy
// analytics derived from the loaded deck corpus. 404 when the card
// isn't in the loaded oracle DB.
func (h *Handler) handleCardByName(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("name")
	name, err := url.PathUnescape(raw)
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "invalid name")
		return
	}
	key := strings.ToLower(strings.TrimSpace(name))
	info, ok := h.cardDB[key]
	if !ok {
		writeError(w, http.StatusNotFound, "card not found")
		return
	}

	display := titleCaseCardName(key)
	usage := h.aggregateCardUsage(key)

	detail := CardDetail{
		Name:             display,
		ManaCost:         info.ManaCost,
		TypeLine:         info.TypeLine,
		OracleText:       info.OracleText,
		CMC:              info.CMC,
		Set:              info.Set,
		ImageURI:         cardImageURI(display),
		Printings:        []CardPrinting{},
		DecksUsing:       usage.decks,
		DeckCount:        len(usage.decks),
		SynergyPartners:  usage.partners,
		RoleDistribution: usage.roles,
		AvgDeckBracket:   usage.avgBracket,
	}
	if info.Set != "" {
		detail.Printings = append(detail.Printings, CardPrinting{
			Set:      info.Set,
			ImageURI: detail.ImageURI,
		})
	}
	writeJSON(w, detail)
}

// cardUsageAggregate bundles the four analytics that depend on a single
// pass over the deck corpus, so we don't re-walk DecksDir per field.
type cardUsageAggregate struct {
	decks      []DeckRef
	partners   []SynergyPartner
	roles      map[string]int
	avgBracket float64
}

// aggregateCardUsage walks DecksDir once and computes everything the
// card detail endpoint needs about how the corpus uses this card.
//
// For each deck whose card list contains cardKey it:
//   - records a DeckRef (with Freya role for the target card)
//   - tallies the deck's bracket into an average
//   - increments role_distribution by the target card's role
//   - increments synergy counts for every OTHER card in the deck
//
// strategy.json is read at most once per matching deck (cached locally).
// Substring quick-reject before parsing keeps this cheap on cold cache.
func (h *Handler) aggregateCardUsage(cardKey string) cardUsageAggregate {
	out := cardUsageAggregate{
		decks: []DeckRef{},
		roles: map[string]int{},
	}
	if h.DecksDir == "" || cardKey == "" {
		return out
	}
	entries, err := os.ReadDir(h.DecksDir)
	if err != nil {
		return out
	}

	// partner-name (lowercased) → count + sample deck display names
	partnerCount := map[string]int{}
	partnerDecks := map[string][]string{}
	const maxSampleDecks = 5

	bracketSum := 0
	bracketN := 0

	for _, ownerEntry := range entries {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		switch owner {
		case "freya", "benched", "test", "moxfield_300", ".versions":
			continue
		}

		deckDir := filepath.Join(h.DecksDir, owner)
		files, err := os.ReadDir(deckDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			fname := f.Name()
			ext := filepath.Ext(fname)
			if ext != ".txt" && ext != ".json" {
				continue
			}
			id := strings.TrimSuffix(fname, ext)

			data, err := os.ReadFile(filepath.Join(deckDir, fname))
			if err != nil {
				continue
			}
			lower := strings.ToLower(string(data))
			if !strings.Contains(lower, cardKey) {
				continue
			}

			var cards []map[string]any
			if ext == ".json" {
				cards = parseDeckJSON(data)
			} else {
				cards = parseDeckList(string(data))
			}
			if !parsedDeckHasCard(cards, cardKey) {
				continue
			}

			meta := loadStrategyMeta(h.DecksDir, owner, id)
			role := ""
			if meta != nil {
				for k, v := range meta.CardRoles {
					if strings.ToLower(k) == cardKey {
						role = v
						break
					}
				}
				if meta.Bracket > 0 {
					bracketSum += meta.Bracket
					bracketN++
				}
			}
			if role == "" {
				out.roles["unassigned"]++
			} else {
				out.roles[role]++
			}

			displayName, _, _, cmdrCard := resolveDeckMetadata(h.DecksDir, owner, id, filepath.Join(deckDir, fname))
			deckLabel := displayName
			if deckLabel == "" {
				deckLabel = id
			}
			out.decks = append(out.decks, DeckRef{
				Owner:     owner,
				ID:        id,
				Name:      displayName,
				Commander: cmdrCard,
				Role:      role,
			})

			// Synergy: every OTHER card in this deck gets a co-occurrence
			// tally. Quantity is intentionally ignored — running 4 of a
			// card in Commander is illegal anyway, and we don't want
			// alternate-art duplicates to skew partner counts.
			seen := map[string]bool{cardKey: true}
			for _, c := range cards {
				raw, _ := c["name"].(string)
				clean := strings.ToLower(strings.TrimSpace(raw))
				if idx := strings.Index(clean, "("); idx > 0 {
					clean = strings.TrimSpace(clean[:idx])
				}
				if clean == "" || seen[clean] {
					continue
				}
				seen[clean] = true
				partnerCount[clean]++
				if len(partnerDecks[clean]) < maxSampleDecks {
					partnerDecks[clean] = append(partnerDecks[clean], deckLabel)
				}
			}
		}
	}

	sort.Slice(out.decks, func(i, j int) bool {
		if out.decks[i].Owner != out.decks[j].Owner {
			return out.decks[i].Owner < out.decks[j].Owner
		}
		return out.decks[i].ID < out.decks[j].ID
	})

	// Top 10 partners.
	out.partners = make([]SynergyPartner, 0, len(partnerCount))
	for name, count := range partnerCount {
		out.partners = append(out.partners, SynergyPartner{
			Name:  titleCaseCardName(name),
			Count: count,
			Decks: partnerDecks[name],
		})
	}
	sort.Slice(out.partners, func(i, j int) bool {
		if out.partners[i].Count != out.partners[j].Count {
			return out.partners[i].Count > out.partners[j].Count
		}
		return out.partners[i].Name < out.partners[j].Name
	})
	if len(out.partners) > 10 {
		out.partners = out.partners[:10]
	}

	if bracketN > 0 {
		out.avgBracket = float64(bracketSum) / float64(bracketN)
		// Round to one decimal so the wire value matches what the UI
		// would render anyway, and so equality compares are stable.
		out.avgBracket = float64(int(out.avgBracket*10+0.5)) / 10.0
	}

	return out
}

// parsedDeckHasCard returns true if the parsed deck contains a card whose
// name (with parenthetical set markers stripped) matches cardKey.
func parsedDeckHasCard(cards []map[string]any, cardKey string) bool {
	for _, c := range cards {
		raw, _ := c["name"].(string)
		clean := strings.ToLower(strings.TrimSpace(raw))
		if idx := strings.Index(clean, "("); idx > 0 {
			clean = strings.TrimSpace(clean[:idx])
		}
		if clean == cardKey {
			return true
		}
	}
	return false
}

// loadStrategyMeta reads <decksDir>/<owner>/freya/<id>.strategy.json and
// returns the subset of fields we use for card analytics. Returns nil
// when the file is missing or unreadable.
func loadStrategyMeta(decksDir, owner, id string) *strategyMeta {
	p := filepath.Join(decksDir, owner, "freya", id+".strategy.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var s strategyMeta
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}
