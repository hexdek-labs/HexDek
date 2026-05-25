package hexapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/versioning"
)

type Handler struct {
	DecksDir      string
	Showmatch     *Showmatch
	IndexHTMLPath string
	cardDB        map[string]oracleCard
	db            *sql.DB // optional — used for deck_meta (custom name, etc.)
	ownerAliases  map[string]string // email prefix → owner slug

	// FeedbackLimiter rate-limits POST /api/feedback per client IP. The
	// endpoint is unauthenticated and writes a file to disk per
	// request, so without a limiter a single bot can fill the
	// feedback dir indefinitely. Nil = no limiting (backwards
	// compatible with binaries that don't yet construct a limiter);
	// cmd/hexdek-server sets a default (5-burst, 1/min refill) at
	// startup.
	FeedbackLimiter *RateLimiter

	// DeckImportLimiter rate-limits the anonymous deck-write endpoints
	// per client IP: POST /api/decks, POST /api/decks/import, POST
	// /api/import/moxfield, and POST /api/decks/{owner}/{id}/analyze.
	// Each call writes a file under DecksDir or kicks off an external
	// fetch + Freya subprocess (analyze) — unbounded calls let an
	// attacker fill disk and saturate the analyze mutex. Nil = no
	// limiting (backwards-compatible). cmd/hexdek-server sets a
	// default (10-burst, 1 per 30s refill) at startup.
	DeckImportLimiter *RateLimiter

	// CSRFStore mints and verifies stateless CSRF tokens. When non-nil
	// the destructive endpoints (DELETE deck, POST clone) require a
	// valid X-CSRF-Token header sourced from GET /api/csrf. When nil
	// the wrapper is a pass-through — preserves backwards compatibility
	// with binaries that haven't constructed the store yet, and lets
	// cmd/hexdek-server gate enforcement on HEXDEK_CSRF_ENFORCE so the
	// existing React SPA keeps working until it's updated to issue +
	// echo the token.
	CSRFStore *CSRFStore

	// DeckMutationLimiter is the first PER-USER (not per-IP) limiter.
	// Buckets by X-HexDek-Owner so the rate budget follows the user
	// across devices and networks. Applied to the owner-authenticated
	// deck mutation endpoints (PUT/PATCH/DELETE /api/decks/{owner}/{id})
	// which previously had no rate limit at all. Per-IP keying was
	// wrong here for two reasons: (1) a household sharing one IP would
	// cross-throttle (alice's bulk edits would lock bob out); (2) a
	// single user with a phone + laptop on different networks would
	// get two separate budgets for what's actually one human's edit
	// session. Falls back to per-IP bucketing when the owner header
	// is missing — see ownerOrIPKey. Nil = no limiting (backwards
	// compatible); cmd/hexdek-server sets a default at startup.
	DeckMutationLimiter *RateLimiter

	deckSubsMu sync.RWMutex
	deckSubs   map[string]map[chan deckEvent]struct{}

	// snapshotCache caches parsed heimdall.GameObservationSnapshot
	// values for /api/games/{id}/summary + /api/games/{id}/summary.pdf
	// + /api/games/{id}/replay. Lazily initialized on first read via
	// ensureSnapshotCache so existing constructors don't need to know
	// about it. Versioned by showmatch_game_observation.created_at so
	// upserts naturally invalidate cached entries.
	snapshotCache     *snapshotCache
	snapshotCacheOnce sync.Once
}

type deckEvent struct {
	Event string
	Data  string
}

type oracleCard struct {
	CMC        float64 `json:"cmc"`
	ManaCost   string  `json:"mana_cost"`
	TypeLine   string  `json:"type_line"`
	OracleText string  `json:"oracle_text"`
	Set        string  `json:"set,omitempty"`
}

func (h *Handler) LoadCardDB(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("carddb: %v", err)
		return
	}
	var cards []struct {
		Name       string  `json:"name"`
		CMC        float64 `json:"cmc"`
		ManaCost   string  `json:"mana_cost"`
		TypeLine   string  `json:"type_line"`
		OracleText string  `json:"oracle_text"`
		Set        string  `json:"set"`
	}
	if err := json.Unmarshal(data, &cards); err != nil {
		log.Printf("carddb: parse error: %v", err)
		return
	}
	h.cardDB = make(map[string]oracleCard, len(cards))
	for _, c := range cards {
		h.cardDB[strings.ToLower(c.Name)] = oracleCard{
			CMC:        c.CMC,
			ManaCost:   c.ManaCost,
			TypeLine:   c.TypeLine,
			OracleText: c.OracleText,
			Set:        c.Set,
		}
	}
	log.Printf("carddb: loaded %d cards", len(h.cardDB))
}

func (h *Handler) LoadOwnerAliases(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		h.ownerAliases = make(map[string]string)
		log.Printf("owner-aliases: no file at %s, starting empty", path)
		return
	}
	var aliases map[string]string
	if err := json.Unmarshal(data, &aliases); err != nil {
		log.Printf("owner-aliases: parse error: %v", err)
		h.ownerAliases = make(map[string]string)
		return
	}
	h.ownerAliases = make(map[string]string, len(aliases))
	for k, v := range aliases {
		h.ownerAliases[strings.ToLower(k)] = strings.ToLower(v)
	}
	log.Printf("owner-aliases: loaded %d mappings", len(h.ownerAliases))
}

func (h *Handler) handleResolveOwner(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email required")
		return
	}
	prefix := strings.ToLower(strings.Split(email, "@")[0])
	prefix = strings.Split(prefix, ".")[0]

	owner := prefix
	if mapped, ok := h.ownerAliases[prefix]; ok {
		owner = mapped
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"owner": owner})
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/decks", h.handleListDecks)
	mux.HandleFunc("POST /api/decks", h.handleImportDeck)
	// Alias for the explicit "import a deck list" flow (Import.jsx page).
	// Same payload + handler as POST /api/decks; the dedicated path lets
	// the dashboard quick-import (modal) and the full-page import flow
	// be tracked separately if we ever split metrics.
	mux.HandleFunc("POST /api/decks/import", h.handleImportDeck)
	mux.HandleFunc("GET /api/decks/{owner}/{id}", h.handleGetDeck)
	mux.HandleFunc("PUT /api/decks/{owner}/{id}", h.handleUpdateDeck)
	mux.HandleFunc("PATCH /api/decks/{owner}/{id}", h.handlePatchDeck)
	// CSRF-gated when h.CSRFStore != nil; pass-through otherwise. Both
	// endpoints below are destructive / state-creating with no second
	// chance, so they're the natural first wave for token enforcement.
	mux.HandleFunc("DELETE /api/decks/{owner}/{id}", RequireCSRF(h.CSRFStore, h.handleDeleteDeck))
	mux.HandleFunc("GET /api/decks/{owner}/{id}/versions", h.handleListVersions)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/versions/{version}", h.handleGetVersion)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/version-trends", h.handleDeckVersionTrends)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/archive", h.handleDeckArchive)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/analysis", h.handleGetAnalysis)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/matchups", h.handleDeckMatchups)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/elo-history", h.handleDeckEloHistory)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/upgrade", h.handleDeckUpgrade)
	mux.HandleFunc("POST /api/decks/{owner}/{id}/analyze", h.handleRunAnalysis)
	mux.HandleFunc("POST /api/decks/{owner}/{id}/clone", RequireCSRF(h.CSRFStore, h.handleCloneDeck))
	// SPA share page with OG meta injection — Caddy can route /decks/{owner}/{id}
	// here for crawler User-Agents (or unconditionally) so Discord/Twitter unfurls
	// pick up per-deck previews.
	mux.HandleFunc("GET /decks/{owner}/{id}", h.handleDeckSharePage)
	mux.HandleFunc("GET /share/{owner}/{id}", h.handleShareDeckPage)
	mux.HandleFunc("GET /cards/{name}", h.handleCardSharePage)
	mux.HandleFunc("GET /operator/{owner}", h.handleOperatorSharePage)
	mux.HandleFunc("GET /spectate", h.handleSpectateSharePage)
	mux.HandleFunc("GET /leaderboard", h.handleLeaderboardSharePage)
	mux.HandleFunc("GET /api/profile", h.handleProfile)
	mux.HandleFunc("GET /api/profile/{owner}", h.handleOwnerProfile)
	mux.HandleFunc("GET /api/profiles", h.handleOwnerProfilesBatch)
	mux.HandleFunc("GET /api/card-art/{name}", h.handleCardArt)
	mux.HandleFunc("GET /api/card-stats/{commander}", h.handleCardWinStats)
	mux.HandleFunc("GET /api/cards/{name}/stats", h.handleCardStats)
	mux.HandleFunc("GET /api/cards/{name}/performance", h.handleCardPerformance)
	mux.HandleFunc("GET /api/meta", h.handleMeta)
	mux.HandleFunc("GET /api/meta/trends", h.handleMetaTrends)
	mux.HandleFunc("GET /api/meta/archetype-vs-archetype", h.handleArchetypeMatrix)
	mux.HandleFunc("GET /api/rivalry/{owner}/{id}", h.handleRivalry)
	mux.HandleFunc("GET /api/threat-graph/{owner}/{id}", h.handleThreatGraph)
	mux.HandleFunc("GET /api/leaderboard", h.handleLeaderboard)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/lineage", h.handleDeckLineage)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/similar", h.handleSimilarDecks)
	mux.HandleFunc("POST /api/import/moxfield", h.handleMoxfieldImport)
	mux.HandleFunc("GET /api/imports/{owner}", h.handleListImports)
	mux.HandleFunc("GET /api/imports/source/moxfield", h.handleMoxfieldSources)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/events", h.handleDeckEvents)
	mux.HandleFunc("POST /api/feedback", h.handleFeedback)
	mux.HandleFunc("POST /api/kofi/webhook", h.handleKofiWebhook)
	mux.HandleFunc("GET /api/donations/summary", h.handleDonationsSummary)
	mux.HandleFunc("GET /api/search", h.handleSearch)
	mux.HandleFunc("GET /api/cards/search", h.handleCardSearch)
	mux.HandleFunc("GET /api/cards/{name}", h.handleCardByName)
	mux.HandleFunc("GET /api/card-stats/card/{cardName}/by-commander", h.handleCardStatsByCommander)
	mux.HandleFunc("GET /api/card-stats/card/{cardName}", h.handleCardStatsOverview)
	mux.HandleFunc("GET /api/deck-card-stats/{owner}/{id}", h.handleDeckCardStats)
	mux.HandleFunc("GET /api/analytics/cards", h.handleCardAnalytics)
	mux.HandleFunc("POST /api/telemetry/pageview", h.handlePageview)
	mux.HandleFunc("POST /api/telemetry/stitch", h.handleStitch)
	mux.HandleFunc("GET /api/resolve-owner", h.handleResolveOwner)
	mux.HandleFunc("GET /api/tags", h.handleListTags)
	mux.HandleFunc("GET /api/players/{id}/trends", h.handlePlayerTrends)
	mux.HandleFunc("GET /api/players/compare", h.handlePlayerCompare)
	mux.HandleFunc("GET /api/tournament/{id}/stats", h.handleTournamentStats)
	mux.HandleFunc("GET /api/games/{id}/summary", h.handleGameSummary)
	mux.HandleFunc("GET /api/games/{id}/replay", h.handleGameReplay)
	mux.HandleFunc("GET /api/games/{id}/summary.pdf", h.handleGameSummaryPDF)
	mux.HandleFunc("GET /api/games/summaries", h.handleGameSummaryArchive)
	mux.HandleFunc("GET /api/games/compare", h.handleGameCompare)
	// CSRF token issuance — always registered. Returns 503 when the
	// store is nil so clients can detect that the server isn't
	// participating in token-based CSRF and fall back to the existing
	// custom-header-only protection.
	mux.HandleFunc("GET /api/csrf", HandleIssueCSRF(h.CSRFStore))
	// Webhook subscriptions — owner-registered HTTP callbacks for
	// engine events (currently just game.end). Returns 503 when h.db
	// is nil so the registry can detect that no SQLite is wired.
	h.RegisterWebhookRoutes(mux)
}

type DeckSummary struct {
	ID               string    `json:"id"`
	Owner            string    `json:"owner"`
	Name             string    `json:"name"`
	Commander        string    `json:"commander"`
	CommanderCard    string    `json:"commander_card,omitempty"`
	CardCount        int       `json:"card_count"`
	Bracket          string    `json:"bracket"`
	Color            string    `json:"color"`
	ImportedAt       time.Time `json:"imported_at"`
	WBS              int       `json:"wbs,omitempty"`
	WBSLabel         string    `json:"wbs_label,omitempty"`
	PLS              int       `json:"pls,omitempty"`
	PLSLabel         string    `json:"pls_label,omitempty"`
	GameChangerCount int       `json:"game_changer_count,omitempty"`
	Archetype        string    `json:"archetype,omitempty"`
	Legal            *bool     `json:"legal,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
}

func (h *Handler) handleListDecks(w http.ResponseWriter, r *http.Request) {
	ownerFilter := r.URL.Query().Get("owner")
	containsFilter := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("contains")))

	decks := []DeckSummary{}
	owners, err := os.ReadDir(h.DecksDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read decks directory")
		return
	}

	for _, ownerEntry := range owners {
		if !ownerEntry.IsDir() {
			continue
		}
		owner := ownerEntry.Name()
		if ownerFilter != "" && owner != ownerFilter {
			continue
		}
		if owner == "freya" || owner == "benched" || owner == "test" || owner == "moxfield_300" || owner == ".versions" {
			continue
		}

		deckDir := filepath.Join(h.DecksDir, owner)
		files, err := os.ReadDir(deckDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || (!strings.HasSuffix(f.Name(), ".txt") && !strings.HasSuffix(f.Name(), ".json")) {
				continue
			}

			name := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			fullPath := filepath.Join(deckDir, f.Name())
			if containsFilter != "" && !deckContainsCard(fullPath, containsFilter) {
				continue
			}
			cards := countCards(fullPath)
			commander, bracket, color, cmdrCard := resolveDeckMetadata(h.DecksDir, owner, name, fullPath)

			var modTime time.Time
			if info, err := f.Info(); err == nil {
				modTime = info.ModTime()
			}

			decks = append(decks, DeckSummary{
				ID:            name,
				Owner:         owner,
				Name:          commander,
				Commander:     commander,
				CommanderCard: cmdrCard,
				CardCount:     cards,
				Bracket:       bracket,
				Color:      color,
				ImportedAt: modTime,
			})
		}
	}

	for i := range decks {
		enrichDeckSummary(h.DecksDir, &decks[i])
		if custom := h.loadCustomName(r.Context(), decks[i].Owner, decks[i].ID); custom != "" {
			decks[i].Name = custom
		}
		decks[i].Tags = h.loadTags(r.Context(), decks[i].Owner, decks[i].ID)
	}

	sort.Slice(decks, func(i, j int) bool {
		return decks[i].ImportedAt.After(decks[j].ImportedAt)
	})

	writeJSON(w, decks)
}

func enrichDeckSummary(decksDir string, ds *DeckSummary) {
	strategyFile := filepath.Join(decksDir, ds.Owner, "freya", ds.ID+".strategy.json")
	data, err := os.ReadFile(strategyFile)
	if err != nil {
		return
	}
	var strat struct {
		Bracket          int    `json:"bracket"`
		BracketLabel     string `json:"bracket_label"`
		PlaysLike        int    `json:"plays_like"`
		PlaysLikeLabel   string `json:"plays_like_label"`
		GameChangerCount int    `json:"game_changer_count"`
		Archetype        string `json:"archetype"`
		Legality         *struct {
			Valid bool `json:"valid"`
		} `json:"legality"`
	}
	if json.Unmarshal(data, &strat) != nil {
		return
	}
	if strat.Bracket > 0 {
		ds.WBS = strat.Bracket
		ds.WBSLabel = strat.BracketLabel
	}
	if strat.PlaysLike > 0 {
		ds.PLS = strat.PlaysLike
		ds.PLSLabel = strat.PlaysLikeLabel
	}
	ds.GameChangerCount = strat.GameChangerCount
	if strat.Archetype != "" {
		ds.Archetype = strat.Archetype
	}
	if strat.Legality != nil {
		v := strat.Legality.Valid
		ds.Legal = &v
	}
}

func (h *Handler) handleGetDeck(w http.ResponseWriter, r *http.Request) {
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
	commander, bracket, color, cmdrCard := resolveDeckMetadata(h.DecksDir, owner, id, deckPath)

	totalCards := 0
	for _, c := range cards {
		if q, ok := c["quantity"].(int); ok {
			totalCards += q
		} else {
			totalCards++
		}
		if h.cardDB != nil {
			name, _ := c["name"].(string)
			lookupName := name
			if idx := strings.Index(lookupName, "("); idx > 0 {
				lookupName = strings.TrimSpace(lookupName[:idx])
			}
			if oc, ok := h.cardDB[strings.ToLower(lookupName)]; ok {
				if _, hasCmc := c["cmc"]; !hasCmc {
					c["cmc"] = int(oc.CMC)
				}
				if _, hasMana := c["mana_cost"]; !hasMana && oc.ManaCost != "" {
					c["mana_cost"] = oc.ManaCost
				}
				if _, hasType := c["type_line"]; !hasType && oc.TypeLine != "" {
					c["type_line"] = oc.TypeLine
				}
			}
		}
	}
	production := computeManaProduction(h.cardDB, cards)
	customName := h.loadCustomName(r.Context(), owner, id)
	clonedFrom := h.loadClonedFrom(r.Context(), owner, id)
	tags := h.loadTags(r.Context(), owner, id)
	writeJSON(w, map[string]any{
		"id":              id,
		"owner":           owner,
		"commander":       commander,
		"commander_card":  cmdrCard,
		"custom_name":     customName,
		"cloned_from":     clonedFrom,
		"bracket":         bracket,
		"color":           color,
		"card_count":      totalCards,
		"cards":           cards,
		"mana_production": production,
		"tags":            tags,
	})
}

func computeManaProduction(cardDB map[string]oracleCard, cards []map[string]any) map[string]int {
	production := map[string]int{}
	basicMap := map[string]string{"plains": "W", "island": "U", "swamp": "B", "mountain": "R", "forest": "G"}
	anyColorPhrases := []string{
		"add one mana of any color",
		"add one mana of any type",
		"adds one mana of any color",
		"add two mana of any",
		"add three mana of any",
		"any combination of colors",
		"mana of any color",
	}

	for _, c := range cards {
		qty := 1
		if q, ok := c["quantity"].(int); ok {
			qty = q
		}
		typeStr := strings.ToLower(fmt.Sprintf("%v", c["type_line"]))
		if !strings.Contains(typeStr, "land") {
			continue
		}

		colored := map[string]bool{}
		for basic, color := range basicMap {
			if strings.Contains(typeStr, basic) {
				colored[color] = true
			}
		}

		if cardDB != nil {
			name, _ := c["name"].(string)
			lookupName := name
			if idx := strings.Index(lookupName, "("); idx > 0 {
				lookupName = strings.TrimSpace(lookupName[:idx])
			}
			if oc, ok := cardDB[strings.ToLower(lookupName)]; ok {
				oracle := strings.ToLower(oc.OracleText)
				for _, phrase := range anyColorPhrases {
					if strings.Contains(oracle, phrase) {
						for _, color := range basicMap {
							colored[color] = true
						}
						break
					}
				}
				for _, pip := range []string{"{w}", "{u}", "{b}", "{r}", "{g}"} {
					colorKey := strings.ToUpper(strings.Trim(pip, "{}"))
					addPattern := "add " + pip
					if strings.Contains(oracle, addPattern) || strings.Contains(oracle, "adds "+pip) {
						colored[colorKey] = true
					}
				}
			}
		}

		for color := range colored {
			production[color] += qty
		}
	}
	return production
}

func (h *Handler) handleUpdateDeck(w http.ResponseWriter, r *http.Request) {
	// Per-user rate-limit. Replaces a deck file on every call; bursts
	// fan into versioning + disk churn. Keyed by owner so alice and
	// bob on the same household IP don't cross-throttle.
	if enforceRateLimitByOwner(h.DeckMutationLimiter, w, r, "deck update") {
		return
	}
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	if !checkOwnership(r, owner) {
		writeError(w, http.StatusForbidden, "forbidden: not deck owner")
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		DeckList string `json:"deck_list"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.DeckList) == "" {
		writeError(w, http.StatusBadRequest, "deck_list is required")
		return
	}

	// Archive current version before overwriting
	versionsDir := filepath.Join(h.DecksDir, owner, "versions")
	os.MkdirAll(versionsDir, 0755)
	ext := filepath.Ext(deckPath)
	ver := 1
	for {
		vPath := filepath.Join(versionsDir, fmt.Sprintf("%s_v%d%s", id, ver, ext))
		if _, err := os.Stat(vPath); os.IsNotExist(err) {
			if oldData, err := os.ReadFile(deckPath); err == nil {
				os.WriteFile(vPath, oldData, 0644)
			}
			break
		}
		ver++
		if ver > 500 {
			break
		}
	}

	if err := os.WriteFile(deckPath, []byte(req.DeckList), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot write deck file")
		return
	}

	cards := parseDeckList(req.DeckList)
	commander, bracket, color, cmdrCard := resolveDeckMetadata(h.DecksDir, owner, id, deckPath)

	// Register new version in the DAG with prior inheritance.
	var cardNames []string
	for _, c := range cards {
		if n, ok := c["name"].(string); ok {
			cardNames = append(cardNames, n)
		}
	}
	go h.registerDeckVersion(owner, id, cmdrCard, cardNames)

	// Auto-trigger Freya analysis on every deck update.
	h.publishDeck(owner+"/"+id, deckEvent{Event: "freya_started", Data: `{"status":"analyzing"}`})
	go h.runFreya(deckPath)

	writeJSON(w, map[string]any{
		"id":             id,
		"owner":          owner,
		"commander":      commander,
		"commander_card": cmdrCard,
		"bracket":        bracket,
		"color":          color,
		"card_count":     len(cards),
		"version":        ver + 1,
	})
}

func (h *Handler) handleDeleteDeck(w http.ResponseWriter, r *http.Request) {
	// Per-user rate-limit on top of CSRF + ownership. Destructive ops
	// shouldn't run in a tight loop even from an authenticated owner.
	if enforceRateLimitByOwner(h.DeckMutationLimiter, w, r, "deck delete") {
		return
	}
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	if !checkOwnership(r, owner) {
		writeError(w, http.StatusForbidden, "forbidden: not deck owner")
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	if err := os.Remove(deckPath); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot delete deck")
		return
	}

	// Clean up Freya analysis if it exists
	strategyFile := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	os.Remove(strategyFile)

	writeJSON(w, map[string]any{"deleted": true, "id": id, "owner": owner})
}

func (h *Handler) handleCloneDeck(w http.ResponseWriter, r *http.Request) {
	srcOwner := r.PathValue("owner")
	srcID := r.PathValue("id")
	if !validatePathComponent(srcOwner) || !validatePathComponent(srcID) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	caller := strings.TrimSpace(strings.ToLower(r.Header.Get("X-HexDek-Owner")))
	if caller == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	dstOwner := sanitizeFilename(caller)
	if dstOwner == "" {
		writeError(w, http.StatusBadRequest, "invalid caller")
		return
	}

	// Rate-limit: at most CloneRateLimit clones per owner per rolling
	// hour. Counted from clone_log; the limiter is permissive when no
	// DB is attached (tests can inject one or accept unlimited).
	since := time.Now().Add(-time.Hour).Unix()
	if n, err := h.cloneCountSince(r.Context(), dstOwner, since); err == nil && n >= CloneRateLimit {
		w.Header().Set("Retry-After", "3600")
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("clone rate limit exceeded (max %d per hour)", CloneRateLimit))
		return
	}

	// Disallow self-cloning — the owner can already edit their own
	// deck, and a "(CLONE)" duplicate in the same collection is just
	// confusing. Other handlers (PATCH, PUT) cover the legitimate
	// "make a copy on my own deck" workflow via versioning.
	if strings.EqualFold(srcOwner, dstOwner) {
		writeError(w, http.StatusBadRequest, "cannot clone your own deck")
		return
	}

	srcPath := findDeckFile(h.DecksDir, srcOwner, srcID)
	if srcPath == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	dstDir := filepath.Join(h.DecksDir, dstOwner)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create deck directory")
		return
	}

	ext := filepath.Ext(srcPath)
	dstID := srcID + "_clone"
	dstPath := filepath.Join(dstDir, dstID+ext)
	for i := 2; ; i++ {
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			break
		}
		dstID = fmt.Sprintf("%s_clone%d", srcID, i)
		dstPath = filepath.Join(dstDir, dstID+ext)
		if i > 100 {
			writeError(w, http.StatusConflict, "too many clones with the same name")
			return
		}
	}

	deckBytes, err := os.ReadFile(srcPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot read source deck")
		return
	}
	if err := os.WriteFile(dstPath, deckBytes, 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot write deck file")
		return
	}

	srcFreyaDir := filepath.Join(h.DecksDir, srcOwner, "freya")
	dstFreyaDir := filepath.Join(dstDir, "freya")
	freyaCopies := map[string]string{
		filepath.Join(srcFreyaDir, srcID+".strategy.json"): filepath.Join(dstFreyaDir, dstID+".strategy.json"),
		filepath.Join(srcFreyaDir, srcID+"_freya.md"):      filepath.Join(dstFreyaDir, dstID+"_freya.md"),
	}
	freyaMade := false
	for src, dst := range freyaCopies {
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		if !freyaMade {
			os.MkdirAll(dstFreyaDir, 0755)
			freyaMade = true
		}
		os.WriteFile(dst, data, 0644)
	}

	var cards []map[string]any
	if strings.HasSuffix(dstPath, ".json") {
		cards = parseDeckJSON(deckBytes)
	} else {
		cards = parseDeckList(string(deckBytes))
	}
	cmdrCard := ""
	for _, line := range strings.Split(string(deckBytes), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMANDER:") {
			cmdrCard = strings.TrimSpace(strings.TrimPrefix(line, "COMMANDER:"))
			break
		}
	}
	var cardNames []string
	for _, c := range cards {
		if n, ok := c["name"].(string); ok {
			cardNames = append(cardNames, n)
		}
	}
	go h.registerDeckVersion(dstOwner, dstID, cmdrCard, cardNames)

	srcCustom := h.loadCustomName(r.Context(), srcOwner, srcID)
	cloneName := srcCustom
	if cloneName == "" {
		cloneName = strings.ToUpper(srcID)
	}
	cloneName = cloneName + " (CLONE)"
	h.saveCustomName(r.Context(), dstOwner, dstID, cloneName)

	srcKey := srcOwner + "/" + srcID
	dstKey := dstOwner + "/" + dstID
	if err := h.saveClonedFrom(r.Context(), dstOwner, dstID, srcKey); err != nil {
		log.Printf("clone: saveClonedFrom failed: %v", err)
	}
	if err := h.recordClone(r.Context(), dstOwner, srcKey, dstKey); err != nil {
		log.Printf("clone: recordClone failed: %v", err)
	}

	h.logImport(r.Context(), db.ImportLogEntry{
		Owner:     dstOwner,
		DeckKey:   dstKey,
		DeckName:  cloneName,
		Commander: cmdrCard,
		Source:    "clone:" + srcKey,
		CardCount: len(cards),
	})

	// Kick off a fresh Freya analysis on the clone. We've already
	// copied the source's strategy.json above so the deck page has
	// something to render immediately; this re-run replaces it with
	// analysis tied to the new deck path so subsequent edits land on
	// a correct baseline. SSE clients see freya_started/_complete.
	h.publishDeck(dstKey, deckEvent{Event: "freya_started", Data: `{"status":"analyzing"}`})
	go h.runFreya(dstPath)

	writeJSON(w, map[string]any{
		"id":             dstID,
		"owner":          dstOwner,
		"name":           cloneName,
		"commander_card": cmdrCard,
		"card_count":     len(cards),
		"source":         srcKey,
		"cloned_from":    srcKey,
	})
}

func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	versionsDir := filepath.Join(h.DecksDir, owner, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		writeJSON(w, []any{})
		return
	}

	prefix := id + "_v"
	var versions []map[string]any
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		vStr := strings.TrimPrefix(base, prefix)
		vNum := parseInt(vStr)
		if vNum == 0 {
			continue
		}
		var modTime time.Time
		if info, err := e.Info(); err == nil {
			modTime = info.ModTime()
		}
		versions = append(versions, map[string]any{
			"version":   vNum,
			"filename":  name,
			"saved_at":  modTime,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i]["version"].(int) > versions[j]["version"].(int)
	})

	writeJSON(w, versions)
}

// handleGetVersion answers GET /api/decks/{owner}/{id}/versions/{version}
// with the raw decklist text + parsed cards for that historical
// snapshot. The DECK HISTORY UI uses this to render a per-version
// detail and to diff two versions client-side.
//
// Returns 404 when the version file is missing — covers both "deck has
// never been edited" (no versions dir) and "version N doesn't exist"
// (gap in the sequence). The decklist text is returned verbatim so
// the client diff renders the same "COMMANDER: X" prefix the saved
// file contains.
func (h *Handler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	versionStr := r.PathValue("version")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	ver := parseInt(versionStr)
	if ver <= 0 || ver > 500 {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}

	versionsDir := filepath.Join(h.DecksDir, owner, "versions")
	// Try both .txt and .json extensions — handleUpdateDeck preserves
	// the source file's extension when archiving, so the version file
	// could be either depending on how the deck was originally imported.
	var data []byte
	var foundPath string
	for _, ext := range []string{".txt", ".json"} {
		candidate := filepath.Join(versionsDir, fmt.Sprintf("%s_v%d%s", id, ver, ext))
		if b, err := os.ReadFile(candidate); err == nil {
			data = b
			foundPath = candidate
			break
		}
	}
	if foundPath == "" {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}

	content := string(data)
	resp := map[string]any{
		"version":   ver,
		"filename":  filepath.Base(foundPath),
		"deck_list": content,
		"cards":     parseDeckList(content),
	}
	if info, err := os.Stat(foundPath); err == nil {
		resp["saved_at"] = info.ModTime()
	}
	writeJSON(w, resp)
}

func (h *Handler) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	freyaDir := filepath.Join(h.DecksDir, owner, "freya")
	strategyFile := filepath.Join(freyaDir, id+".strategy.json")

	data, err := os.ReadFile(strategyFile)
	if err != nil {
		// Auto-trigger Freya analysis if the deck file exists.
		deckPath := findDeckFile(h.DecksDir, owner, id)
		if deckPath != "" {
			go h.runFreya(deckPath)
			writeJSON(w, map[string]any{"status": "analyzing", "message": "Freya analysis started — refresh in a few seconds"})
			return
		}
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handler) handleRunAnalysis(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate-limit. Each call kicks off a Freya subprocess (serialized
	// behind freyaMu, but the queue is unbounded — an attacker spamming
	// this endpoint pins a worker indefinitely). Shares DeckImportLimiter
	// with the other deck-write endpoints so a burst across them sums
	// against the same budget.
	if enforceRateLimit(h.DeckImportLimiter, w, r, "deck analyze") {
		return
	}
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

	go h.runFreya(deckPath)
	writeJSON(w, map[string]any{"status": "analyzing", "deck": owner + "/" + id})
}

var freyaMu sync.Mutex

func (h *Handler) runFreya(deckPath string) {
	freyaMu.Lock()
	defer freyaMu.Unlock()

	freyaBin := "hexdek-freya"
	if _, err := exec.LookPath(freyaBin); err != nil {
		freyaBin = "./hexdek-freya"
		if _, err := os.Stat(freyaBin); err != nil {
			log.Printf("freya: binary not found")
			return
		}
	}

	log.Printf("freya: analyzing %s", deckPath)
	cmd := exec.Command(freyaBin, "--deck", deckPath, "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("freya: error analyzing %s: %v\n%s", deckPath, err, string(out))
		return
	}
	log.Printf("freya: completed %s", deckPath)

	rel, _ := filepath.Rel(h.DecksDir, deckPath)
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if len(parts) == 2 {
		owner := parts[0]
		id := strings.TrimSuffix(parts[1], filepath.Ext(parts[1]))
		h.publishDeck(owner+"/"+id, deckEvent{
			Event: "freya_complete",
			Data:  `{"status":"complete"}`,
		})
	}
}

func (h *Handler) subscribeDeck(key string) chan deckEvent {
	h.deckSubsMu.Lock()
	defer h.deckSubsMu.Unlock()
	if h.deckSubs == nil {
		h.deckSubs = make(map[string]map[chan deckEvent]struct{})
	}
	if h.deckSubs[key] == nil {
		h.deckSubs[key] = make(map[chan deckEvent]struct{})
	}
	ch := make(chan deckEvent, 4)
	h.deckSubs[key][ch] = struct{}{}
	return ch
}

func (h *Handler) unsubscribeDeck(key string, ch chan deckEvent) {
	h.deckSubsMu.Lock()
	defer h.deckSubsMu.Unlock()
	if subs, ok := h.deckSubs[key]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(h.deckSubs, key)
		}
	}
	close(ch)
}

func (h *Handler) publishDeck(key string, ev deckEvent) {
	h.deckSubsMu.RLock()
	defer h.deckSubsMu.RUnlock()
	for ch := range h.deckSubs[key] {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *Handler) handleDeckEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	flusher.Flush()

	key := owner + "/" + id
	ch := h.subscribeDeck(key)
	defer h.unsubscribeDeck(key, ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Event, ev.Data)
			flusher.Flush()
		}
	}
}

func (h *Handler) handleProfile(w http.ResponseWriter, r *http.Request) {
	// Global / unauthenticated profile envelope. Per-owner data lives at
	// /api/profile/{owner}; this endpoint only surfaces server-wide
	// Showmatch stats so the public dashboard has something to render.
	profile := map[string]any{
		"username":     "",
		"userId":       "",
		"joined":       "",
		"elo":          0,
		"eloChange":    0,
		"tier":         "",
		"streak":       "",
		"primaryColor": "",
		"archetype":    "",
		"percentile":   "",
		"gamesPlayed":  0,
		"winRate":      0.0,
		"avgWinTurn":   0.0,
	}

	if h.Showmatch != nil {
		stats := h.Showmatch.GetStats()
		elo := h.Showmatch.GetELO()
		profile["gamesPlayed"] = stats.GamesPlayed
		profile["avgWinTurn"] = stats.AvgTurns
		if len(elo) > 0 {
			top := elo[0]
			profile["elo"] = int(top.Rating)
			profile["eloChange"] = int(top.Delta)
			profile["primaryColor"] = top.Commander
		}
		if stats.Dominant != "" {
			profile["archetype"] = stats.Dominant
		}
		games := h.Showmatch.GetGameHistory(0)
		if len(games) > 0 {
			wins := 0
			for _, g := range games {
				if g.Winner >= 0 {
					wins++
				}
			}
			profile["winRate"] = float64(wins) / float64(len(games)) * 100.0
		}
	}

	writeJSON(w, profile)
}

type ImportRequest struct {
	Name     string   `json:"name"`
	Owner    string   `json:"owner"`
	DeckList string   `json:"deck_list"`
	Tags     []string `json:"tags,omitempty"`
}

func (h *Handler) handleImportDeck(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate-limit. POST /api/decks + POST /api/decks/import both
	// route here; both write a file under DecksDir and parse the body.
	// Unbounded calls let an attacker fill disk with arbitrary deck
	// text. Limiter is nil-safe.
	if enforceRateLimit(h.DeckImportLimiter, w, r, "deck import") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if strings.TrimSpace(req.DeckList) == "" {
		writeError(w, http.StatusBadRequest, "deck_list is required")
		return
	}

	// Default owner
	owner := strings.TrimSpace(req.Owner)
	if owner == "" {
		owner = "imported"
	}
	// Sanitize owner: lowercase, alphanumeric + underscore only
	owner = sanitizeFilename(owner)

	// Default name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "imported_deck"
	}
	// Sanitize name for filename
	fileID := sanitizeFilename(strings.ToLower(name))
	if fileID == "" {
		fileID = "deck"
	}

	// Ensure owner directory exists
	ownerDir := filepath.Join(h.DecksDir, owner)
	if err := os.MkdirAll(ownerDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create deck directory")
		return
	}

	// Write the deck file
	deckPath := filepath.Join(ownerDir, fileID+".txt")
	// If file already exists, append a number
	for i := 2; ; i++ {
		if _, err := os.Stat(deckPath); os.IsNotExist(err) {
			break
		}
		deckPath = filepath.Join(ownerDir, fmt.Sprintf("%s_%d.txt", fileID, i))
		if i > 100 {
			writeError(w, http.StatusConflict, "too many decks with the same name")
			return
		}
	}

	if err := os.WriteFile(deckPath, []byte(req.DeckList), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot write deck file")
		return
	}

	// Parse the deck to return summary
	cards := parseDeckList(req.DeckList)
	cmdrCard := ""
	for _, line := range strings.Split(req.DeckList, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMANDER:") {
			cmdrCard = strings.TrimSpace(strings.TrimPrefix(line, "COMMANDER:"))
			break
		}
	}

	finalID := strings.TrimSuffix(filepath.Base(deckPath), ".txt")

	// Register version in the DAG.
	var cardNames []string
	for _, c := range cards {
		if n, ok := c["name"].(string); ok {
			cardNames = append(cardNames, n)
		}
	}
	go h.registerDeckVersion(owner, finalID, cmdrCard, cardNames)

	if name != "" && name != "imported_deck" {
		h.saveCustomName(r.Context(), owner, finalID, name)
	}

	if len(req.Tags) > 0 {
		if tagsJSON, err := normalizeTags(req.Tags); err == nil && tagsJSON != "" {
			h.saveTags(r.Context(), owner, finalID, tagsJSON)
		}
	}

	h.logImport(r.Context(), db.ImportLogEntry{
		Owner:     owner,
		DeckKey:   owner + "/" + finalID,
		DeckName:  name,
		Commander: cmdrCard,
		Source:    "paste",
		CardCount: len(cards),
	})

	// Auto-trigger Freya analysis on import.
	go h.runFreya(deckPath)

	writeJSON(w, map[string]any{
		"id":             finalID,
		"owner":          owner,
		"name":           name,
		"commander_card": cmdrCard,
		"card_count":     len(cards),
		"file_path":      filepath.Join(owner, filepath.Base(deckPath)),
		"tags":           h.loadTags(r.Context(), owner, finalID),
	})
}

var moxfieldClient = &http.Client{
	Timeout: 20 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		host := req.URL.Hostname()
		if !strings.HasSuffix(host, "moxfield.com") {
			return fmt.Errorf("redirect to disallowed host: %s", host)
		}
		return nil
	},
}

func (h *Handler) handleMoxfieldImport(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate-limit. Each call fans out to moxfield.com (external
	// network cost we pay) and then writes a deck file under DecksDir.
	// Shares DeckImportLimiter with the local-import path.
	if enforceRateLimit(h.DeckImportLimiter, w, r, "moxfield import") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		URL   string   `json:"url"`
		Owner string   `json:"owner"`
		Tags  []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host != "www.moxfield.com" {
		writeError(w, http.StatusBadRequest, "invalid Moxfield URL")
		return
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "decks" {
		writeError(w, http.StatusBadRequest, "URL must be https://www.moxfield.com/decks/{id}")
		return
	}
	moxID := parts[1]

	apiURL := "https://api2.moxfield.com/v3/decks/all/" + url.PathEscape(moxID)
	apiReq, _ := http.NewRequest("GET", apiURL, nil)
	apiReq.Header.Set("User-Agent", "HexDek/1.0 (hexdek deck import)")
	apiReq.Header.Set("Accept", "application/json")
	resp, err := moxfieldClient.Do(apiReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch from Moxfield: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Moxfield returned %d", resp.StatusCode))
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read Moxfield response")
		return
	}

	var moxDeck struct {
		Name       string `json:"name"`
		Format     string `json:"format"`
		Commanders map[string]struct {
			Card struct {
				Name string `json:"name"`
			} `json:"card"`
			Quantity int `json:"quantity"`
		} `json:"commanders"`
		Mainboard map[string]struct {
			Card struct {
				Name string `json:"name"`
			} `json:"card"`
			Quantity int `json:"quantity"`
		} `json:"mainboard"`
	}
	if err := json.Unmarshal(body, &moxDeck); err != nil {
		writeError(w, http.StatusBadGateway, "failed to parse Moxfield response")
		return
	}

	var lines []string
	var cmdrName string
	var cardNames []string
	for _, c := range moxDeck.Commanders {
		lines = append(lines, "COMMANDER: "+c.Card.Name)
		if cmdrName == "" {
			cmdrName = c.Card.Name
		}
		cardNames = append(cardNames, c.Card.Name)
	}
	for _, c := range moxDeck.Mainboard {
		lines = append(lines, fmt.Sprintf("%d %s", c.Quantity, c.Card.Name))
		cardNames = append(cardNames, c.Card.Name)
	}
	deckList := strings.Join(lines, "\n")

	owner := sanitizeFilename(strings.TrimSpace(req.Owner))
	if owner == "" {
		owner = "imported"
	}

	deckName := moxDeck.Name
	if deckName == "" {
		deckName = cmdrName
	}
	fileID := sanitizeFilename(strings.ToLower(deckName))
	if fileID == "" {
		fileID = "moxfield_deck"
	}

	ownerDir := filepath.Join(h.DecksDir, owner)
	if err := os.MkdirAll(ownerDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot create deck directory")
		return
	}

	deckPath := filepath.Join(ownerDir, fileID+".txt")
	for i := 2; ; i++ {
		if _, err := os.Stat(deckPath); os.IsNotExist(err) {
			break
		}
		deckPath = filepath.Join(ownerDir, fmt.Sprintf("%s_%d.txt", fileID, i))
		if i > 100 {
			writeError(w, http.StatusConflict, "too many decks with the same name")
			return
		}
	}

	if err := os.WriteFile(deckPath, []byte(deckList), 0644); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot write deck file")
		return
	}

	finalID := strings.TrimSuffix(filepath.Base(deckPath), ".txt")
	go h.registerDeckVersion(owner, finalID, cmdrName, cardNames)

	if len(req.Tags) > 0 {
		if tagsJSON, err := normalizeTags(req.Tags); err == nil && tagsJSON != "" {
			h.saveTags(r.Context(), owner, finalID, tagsJSON)
		}
	}

	h.logImport(r.Context(), db.ImportLogEntry{
		Owner:     owner,
		DeckKey:   owner + "/" + finalID,
		DeckName:  deckName,
		Commander: cmdrName,
		Source:    "moxfield",
		SourceURL: req.URL,
		CardCount: len(cardNames),
	})

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"id":          finalID,
		"owner":       owner,
		"name":        deckName,
		"commander":   cmdrName,
		"card_count":  len(cardNames),
		"source":      "moxfield",
		"moxfield_id": moxID,
		"tags":        h.loadTags(r.Context(), owner, finalID),
	})
}

// logImport persists a deck-import event. Best-effort: a missing or failing
// SQLite layer must not fail the import itself, so errors are logged only.
func (h *Handler) logImport(ctx context.Context, e db.ImportLogEntry) {
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		return
	}
	if _, err := db.InsertImportLog(ctx, h.Showmatch.sqlDB, e); err != nil {
		log.Printf("import_log: insert failed: %v", err)
	}
}

func (h *Handler) handleListImports(w http.ResponseWriter, r *http.Request) {
	owner := sanitizeFilename(strings.TrimSpace(r.PathValue("owner")))
	if owner == "" {
		writeError(w, http.StatusBadRequest, "owner required")
		return
	}
	limit := 25
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{"owner": owner, "imports": []any{}})
		return
	}
	entries, err := db.ListImportLogs(r.Context(), h.Showmatch.sqlDB, owner, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if entries == nil {
		entries = []db.ImportLogEntry{}
	}
	writeJSON(w, map[string]any{"owner": owner, "imports": entries})
}

// handleMoxfieldSources returns every deck on the system whose origin we
// can trace back to Moxfield. Two complementary sources:
//
//  1. import_log rows where source = "moxfield" (per-user uploads via the
//     /api/import/moxfield endpoint), which include the full source_url.
//  2. Bulk-imported deck files in data/decks/moxfield* whose filename
//     convention encodes the Moxfield deck ID as the suffix after the
//     last underscore — we derive the URL from that.
//
// Returned list is the union of both, with `source` distinguishing
// "import_log" rows from "filename_convention" rows. Optional ?limit
// caps the total; defaults to 500.
func (h *Handler) handleMoxfieldSources(w http.ResponseWriter, r *http.Request) {
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 5000 {
			limit = n
		}
	}

	type entry struct {
		Owner       string `json:"owner"`
		DeckID      string `json:"deck_id"`
		DeckKey     string `json:"deck_key"`
		Commander   string `json:"commander,omitempty"`
		MoxfieldID  string `json:"moxfield_id,omitempty"`
		MoxfieldURL string `json:"moxfield_url"`
		Source      string `json:"source"` // "import_log" | "filename_convention"
		ImportedAt  int64  `json:"imported_at,omitempty"`
	}
	out := make([]entry, 0, 256)

	// 1) import_log rows (source = "moxfield")
	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		rows, err := h.Showmatch.sqlDB.QueryContext(r.Context(),
			`SELECT owner, deck_key, COALESCE(commander,''), COALESCE(source_url,''), imported_at
			 FROM import_log WHERE source = 'moxfield'
			 ORDER BY imported_at DESC`)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var e entry
				var url string
				if err := rows.Scan(&e.Owner, &e.DeckKey, &e.Commander, &url, &e.ImportedAt); err != nil {
					continue
				}
				e.MoxfieldURL = url
				// deck_id is the slug after "owner/"
				parts := strings.SplitN(e.DeckKey, "/", 2)
				if len(parts) == 2 {
					e.DeckID = parts[1]
				}
				e.Source = "import_log"
				out = append(out, e)
			}
		}
	}

	// 2) filename-convention scan of bulk moxfield directories
	for _, sub := range []string{"moxfield", "moxfield_300"} {
		dir := filepath.Join(h.DecksDir, sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, de := range entries {
			if de.IsDir() {
				continue
			}
			name := de.Name()
			if !strings.HasSuffix(name, ".txt") {
				continue
			}
			base := strings.TrimSuffix(name, ".txt")
			lastUnderscore := strings.LastIndex(base, "_")
			if lastUnderscore < 0 {
				continue
			}
			moxID := base[lastUnderscore+1:]
			if len(moxID) < 4 {
				continue
			}
			out = append(out, entry{
				Owner:       sub,
				DeckID:      base,
				DeckKey:     sub + "/" + base,
				MoxfieldID:  moxID,
				MoxfieldURL: "https://www.moxfield.com/decks/" + moxID,
				Source:      "filename_convention",
			})
		}
	}

	if len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, map[string]any{
		"total":   len(out),
		"sources": out,
	})
}

func (h *Handler) handleCardWinStats(w http.ResponseWriter, r *http.Request) {
	commander := r.PathValue("commander")
	if commander == "" {
		writeError(w, http.StatusBadRequest, "commander required")
		return
	}
	commander = strings.ReplaceAll(commander, "_", " ")
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{"cards": []any{}, "commander": commander})
		return
	}
	stats, err := db.LoadCardWinStats(r.Context(), h.Showmatch.sqlDB, commander, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	writeJSON(w, map[string]any{
		"commander": commander,
		"cards":     stats,
	})
}

// handleDeckMatchups returns this deck's head-to-head record against
// every opposing commander it has met in showmatch_game history. The
// SQL filter is bound as a parameter so injection-safe; the path-
// component validation is defense-in-depth and rejects components
// outside [a-zA-Z0-9_-.] before any DB call.
func (h *Handler) handleDeckMatchups(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{"deck_key": deckKey, "matchups": []any{}})
		return
	}
	rows, err := db.LoadDeckMatchups(r.Context(), h.Showmatch.sqlDB, deckKey, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if rows == nil {
		rows = []db.MatchupRow{}
	}
	writeJSON(w, map[string]any{
		"deck_key": deckKey,
		"matchups": rows,
	})
}

// handleDeckEloHistory returns the most recent gauntlet runs for a deck,
// chronological (oldest first) so the frontend can plot rating-over-time
// without re-sorting. ?limit=N caps the returned series (default 20).
func (h *Handler) handleDeckEloHistory(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}
	deckKey := owner + "/" + id
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{"deck_key": deckKey, "runs": []any{}})
		return
	}
	limit := 20
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	rows, err := db.LoadGauntletRuns(r.Context(), h.Showmatch.sqlDB, deckKey, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	// DB returns newest-first; reverse for chronological display.
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if rows == nil {
		rows = []db.GauntletRunRecord{}
	}
	writeJSON(w, map[string]any{
		"deck_key": deckKey,
		"runs":     rows,
	})
}

func validatePathComponent(s string) bool {
	if s == "" || s == "." || s == ".." || strings.Contains(s, "/") || strings.Contains(s, "\\") || strings.Contains(s, "\x00") {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}

func checkOwnership(r *http.Request, owner string) bool {
	caller := strings.TrimSpace(strings.ToLower(r.Header.Get("X-HexDek-Owner")))
	return caller != "" && caller == strings.ToLower(owner)
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			b.WriteRune(c)
		} else if c == ' ' || c == ',' {
			b.WriteRune('_')
		}
	}
	return b.String()
}

// resolveDeckMetadata returns the metadata used by deck list/get/search
// responses. It starts from parseDeckFilename and falls back to the
// deck's COMMANDER: line and Freya strategy.json bracket when the
// filename doesn't follow the <words>_b<N>_<rest> convention.
//
// fullPath may be empty when only the id is known.
func resolveDeckMetadata(decksDir, owner, id, fullPath string) (commander, bracket, color, cmdrCard string) {
	commander, bracket, color = parseDeckFilename(id)
	if fullPath != "" {
		cmdrCard = extractCommander(fullPath)
	}

	// Filename is the source of truth when it has a _b<N>_ marker;
	// only fall back when it doesn't.
	if bracket != "?" {
		return
	}
	if b := strategyBracket(decksDir, owner, id); b > 0 {
		bracket = strconv.Itoa(b)
	}
	if cmdrCard != "" {
		commander = strings.ToUpper(cmdrCard)
	}
	return
}

func strategyBracket(decksDir, owner, id string) int {
	if decksDir == "" || owner == "" || id == "" {
		return 0
	}
	p := filepath.Join(decksDir, owner, "freya", id+".strategy.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return 0
	}
	var s struct {
		Bracket int `json:"bracket"`
	}
	if json.Unmarshal(data, &s) != nil {
		return 0
	}
	return s.Bracket
}

func parseDeckFilename(name string) (commander, bracket, color string) {
	parts := strings.Split(name, "_")

	bracketIdx := -1
	for i, p := range parts {
		if len(p) == 2 && p[0] == 'b' && p[1] >= '0' && p[1] <= '9' {
			bracket = string(p[1])
			bracketIdx = i
			break
		}
	}
	if bracket == "" {
		bracket = "?"
	}

	var nameParts []string
	if bracketIdx >= 0 {
		nameParts = parts[:bracketIdx]
	} else {
		nameParts = parts
	}

	commander = strings.ToUpper(strings.Join(nameParts, " "))
	if commander == "" {
		commander = strings.ToUpper(strings.ReplaceAll(name, "_", " "))
	}
	color = "?"
	return
}

func extractCommander(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if strings.HasSuffix(path, ".json") {
		type deckJSON struct {
			Commander string `json:"commander"`
		}
		var d deckJSON
		if json.Unmarshal(data, &d) == nil && d.Commander != "" {
			return d.Commander
		}
		return ""
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "COMMANDER:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "COMMANDER:"))
		}
	}
	return ""
}

func parseDeckJSON(data []byte) []map[string]any {
	var deck struct {
		Commander string `json:"commander"`
		Mainboard []struct {
			Name     string   `json:"name"`
			Quantity int      `json:"quantity"`
			ManaCost string   `json:"mana_cost"`
			CMC      int      `json:"cmc"`
			Types    []string `json:"types"`
		} `json:"mainboard"`
	}
	if err := json.Unmarshal(data, &deck); err != nil {
		return nil
	}
	var cards []map[string]any
	cmdrLower := strings.ToLower(deck.Commander)
	cmdrInList := false
	for _, c := range deck.Mainboard {
		entry := map[string]any{
			"name":     c.Name,
			"quantity": c.Quantity,
			"cmc":      c.CMC,
		}
		if c.ManaCost != "" {
			entry["mana_cost"] = c.ManaCost
		}
		if len(c.Types) > 0 {
			entry["type_line"] = strings.Join(c.Types, " ")
		}
		cards = append(cards, entry)
		if cmdrLower != "" && strings.Contains(strings.ToLower(c.Name), cmdrLower) {
			cmdrInList = true
		}
	}
	if deck.Commander != "" && !cmdrInList {
		cards = append(cards, map[string]any{
			"name":     deck.Commander,
			"quantity": 1,
		})
	}
	return cards
}

// deckContainsCard returns true if the deck file at path contains a card
// whose name contains needle (case-insensitive substring match). Needle
// must already be lowercased. Reads commander, partner, and main-list
// card names; ignores quantity prefixes and set codes in parentheses.
func deckContainsCard(path, needle string) bool {
	if needle == "" {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if strings.HasSuffix(path, ".json") {
		for _, c := range parseDeckJSON(data) {
			if name, ok := c["name"].(string); ok && strings.Contains(strings.ToLower(name), needle) {
				return true
			}
		}
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "COMMANDER:") {
			if strings.Contains(strings.ToLower(strings.TrimSpace(line[len("COMMANDER:"):])), needle) {
				return true
			}
			continue
		}
		if strings.HasPrefix(upper, "PARTNER:") {
			if strings.Contains(strings.ToLower(strings.TrimSpace(line[len("PARTNER:"):])), needle) {
				return true
			}
			continue
		}
		name := line
		if parts := strings.SplitN(line, " ", 2); len(parts) == 2 && parseInt(parts[0]) > 0 {
			name = parts[1]
		}
		if idx := strings.Index(name, "("); idx > 0 {
			name = strings.TrimSpace(name[:idx])
		}
		if strings.Contains(strings.ToLower(name), needle) {
			return true
		}
	}
	return false
}

func countCards(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if strings.HasSuffix(path, ".json") {
		cards := parseDeckJSON(data)
		total := 0
		for _, c := range cards {
			if q, ok := c["quantity"].(int); ok {
				total += q
			} else {
				total++
			}
		}
		return total
	}
	count := 0
	var cmdrName, partnerName string
	var cardNames []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "COMMANDER:") {
			cmdrName = strings.TrimSpace(line[len("COMMANDER:"):])
			continue
		}
		if strings.HasPrefix(upper, "PARTNER:") {
			partnerName = strings.TrimSpace(line[len("PARTNER:"):])
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		name := line
		if len(parts) == 2 {
			if n := parseInt(parts[0]); n > 0 {
				count += n
				name = parts[1]
			} else {
				count++
			}
		} else {
			count++
		}
		if idx := strings.Index(name, "("); idx > 0 {
			name = strings.TrimSpace(name[:idx])
		}
		cardNames = append(cardNames, strings.ToLower(name))
	}
	cmdrInList := false
	partnerInList := false
	cmdrLower := strings.ToLower(cmdrName)
	partnerLower := strings.ToLower(partnerName)
	for _, n := range cardNames {
		if cmdrLower != "" && n == cmdrLower {
			cmdrInList = true
		}
		if partnerLower != "" && n == partnerLower {
			partnerInList = true
		}
	}
	if cmdrName != "" && !cmdrInList {
		count++
	}
	if partnerName != "" && !partnerInList {
		count++
	}
	return count
}

func parseDeckList(content string) []map[string]any {
	var cards []map[string]any
	var cmdrName, partnerName string
	cardNameSet := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "COMMANDER:") {
			cmdrName = strings.TrimSpace(line[len("COMMANDER:"):])
			continue
		}
		if strings.HasPrefix(upper, "PARTNER:") {
			partnerName = strings.TrimSpace(line[len("PARTNER:"):])
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		qty := 1
		name := line
		if len(parts) == 2 {
			if n := parseInt(parts[0]); n > 0 {
				qty = n
				name = parts[1]
			}
		}
		stripped := name
		if idx := strings.Index(stripped, "("); idx > 0 {
			stripped = strings.TrimSpace(stripped[:idx])
		}
		cardNameSet[strings.ToLower(stripped)] = true
		cards = append(cards, map[string]any{
			"quantity": qty,
			"name":    name,
		})
	}
	if cmdrName != "" && !cardNameSet[strings.ToLower(cmdrName)] {
		cards = append(cards, map[string]any{
			"quantity": 1,
			"name":    cmdrName,
		})
	}
	if partnerName != "" && !cardNameSet[strings.ToLower(partnerName)] {
		cards = append(cards, map[string]any{
			"quantity": 1,
			"name":    partnerName,
		})
	}
	return cards
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func findDeckFile(base, owner, id string) string {
	dir := filepath.Join(base, owner)
	for _, ext := range []string{".txt", ".json"} {
		path := filepath.Join(dir, id+ext)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

var artCacheDir = filepath.Join("data", "cache", "art")
var artCacheMu sync.Mutex
var artMemCache sync.Map
var artInflight sync.Map

func init() {
	go warmArtCache()
}

func warmArtCache() {
	entries, err := os.ReadDir(artCacheDir)
	if err != nil {
		return
	}
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jpg") {
			continue
		}
		hash := strings.TrimSuffix(e.Name(), ".jpg")
		data, err := os.ReadFile(filepath.Join(artCacheDir, e.Name()))
		if err == nil && len(data) > 0 {
			artMemCache.Store(hash, data)
			loaded++
		}
	}
	log.Printf("art cache: warmed %d images from disk", loaded)
}
var artHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return fmt.Errorf("too many redirects")
		}
		host := req.URL.Hostname()
		if !strings.HasSuffix(host, "scryfall.com") && !strings.HasSuffix(host, "scryfall.io") {
			return fmt.Errorf("redirect to disallowed host: %s", host)
		}
		return nil
	},
}

type artResult struct {
	data []byte
	err  error
}

func (h *Handler) handleCardArt(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing card name")
		return
	}

	version := r.URL.Query().Get("version")
	if version != "normal" && version != "large" && version != "png" {
		version = "art_crop"
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(name)+"|"+version)))

	if cached, ok := artMemCache.Load(hash); ok {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
		w.Write(cached.([]byte))
		return
	}

	cachePath := filepath.Join(artCacheDir, hash+".jpg")
	if data, err := os.ReadFile(cachePath); err == nil {
		artMemCache.Store(hash, data)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
		w.Write(data)
		return
	}

	ch := make(chan artResult, 1)
	if actual, loaded := artInflight.LoadOrStore(hash, ch); loaded {
		res := <-actual.(chan artResult)
		actual.(chan artResult) <- res
		if res.err != nil {
			writeError(w, http.StatusNotFound, "card art not found")
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
		w.Write(res.data)
		return
	}
	defer artInflight.Delete(hash)

	clean := strings.Split(name, "//")[0]
	clean = strings.TrimSpace(clean)
	scryfallURL := "https://api.scryfall.com/cards/named?fuzzy=" + url.QueryEscape(clean) + "&format=image&version=" + version

	req, _ := http.NewRequest("GET", scryfallURL, nil)
	req.Header.Set("User-Agent", "HexDek/1.0 (hexdek card art cache)")
	req.Header.Set("Accept", "image/*")
	resp, err := artHTTPClient.Do(req)
	if err != nil {
		ch <- artResult{err: err}
		writeError(w, http.StatusNotFound, "card art not found")
		return
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		ch <- artResult{err: fmt.Errorf("scryfall %d", resp.StatusCode)}
		writeError(w, http.StatusNotFound, "card art not found")
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		ch <- artResult{err: err}
		writeError(w, http.StatusInternalServerError, "failed to read art")
		return
	}

	artCacheMu.Lock()
	os.MkdirAll(artCacheDir, 0755)
	os.WriteFile(cachePath, data, 0644)
	artCacheMu.Unlock()

	artMemCache.Store(hash, data)
	ch <- artResult{data: data}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=2592000, stale-while-revalidate=86400")
	w.Write(data)
}

func (h *Handler) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	dag, err := versioning.LoadDAG(filepath.Join(h.DecksDir, ".versions"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load version DAG")
		return
	}

	heads := dag.Leaderboard()
	type entry struct {
		Owner       string  `json:"owner"`
		DeckID      string  `json:"deck_id"`
		Commander   string  `json:"commander"`
		Version     int     `json:"version"`
		Hash        string  `json:"hash"`
		Rating      float64 `json:"rating"`
		Mu          float64 `json:"mu"`
		Sigma       float64 `json:"sigma"`
		GamesPlayed int     `json:"games_played"`
	}
	var out []entry
	for _, h := range heads {
		out = append(out, entry{
			Owner:       h.Owner,
			DeckID:      h.DeckID,
			Commander:   h.Commander,
			Version:     h.Version,
			Hash:        h.Hash,
			Rating:      h.Rating.Conservative(),
			Mu:          h.Rating.Mu,
			Sigma:       h.Rating.Sigma,
			GamesPlayed: h.GamesPlayed,
		})
	}
	writeJSON(w, out)
}

func (h *Handler) handleDeckLineage(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	dag, err := versioning.LoadDAG(filepath.Join(h.DecksDir, ".versions"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load version DAG")
		return
	}

	lineage := dag.GetLineage(owner, id)
	if lineage == nil {
		writeJSON(w, []any{})
		return
	}

	writeJSON(w, lineage)
}

// registerDeckVersion records a deck version in the DAG with Bayesian
// prior inheritance. Called on import and update.
func (h *Handler) registerDeckVersion(owner, deckID, commander string, cardNames []string) {
	dagDir := filepath.Join(h.DecksDir, ".versions")
	dag, err := versioning.LoadDAG(dagDir)
	if err != nil {
		log.Printf("versioning: load DAG: %v", err)
		return
	}

	dag.RegisterVersion(owner, deckID, commander, cardNames)

	if err := versioning.SaveDAG(dagDir, dag); err != nil {
		log.Printf("versioning: save DAG: %v", err)
	}
}

func (h *Handler) handleRivalry(w http.ResponseWriter, r *http.Request) {
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

	commander := extractCommander(deckPath)
	if commander == "" {
		commander, _, _ = parseDeckFilename(id)
	}

	rivalries, err := analytics.LoadRivalries("data/rivalry")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load rivalry data")
		return
	}

	top := analytics.TopRivals(rivalries, commander, 10)
	writeJSON(w, map[string]any{
		"commander": commander,
		"owner":     owner,
		"deck_id":   id,
		"rivals":    top,
	})
}

func (h *Handler) handleThreatGraph(w http.ResponseWriter, r *http.Request) {
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

	commander := extractCommander(deckPath)
	if commander == "" {
		commander, _, _ = parseDeckFilename(id)
	}

	edges, err := analytics.LoadThreatGraph("data/analytics")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load threat graph")
		return
	}

	summary := analytics.ThreatSummaryFor(edges, commander, 10)
	writeJSON(w, summary)
}

func (h *Handler) handleFeedback(w http.ResponseWriter, r *http.Request) {
	// Per-IP rate-limit. Anonymous endpoint that writes a file to disk
	// per request — a single bot blasting it would fill the feedback
	// dir. Limiter is nil-safe so older binaries keep working.
	if enforceRateLimit(h.FeedbackLimiter, w, r, "feedback") {
		return
	}
	var body struct {
		Type     string `json:"type"`
		Page     string `json:"page"`
		Context  string `json:"context"`
		Symptom  string `json:"symptom"`
		Expected string `json:"expected"`
		Contact  string `json:"contact"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32768)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if body.Symptom == "" {
		writeError(w, http.StatusBadRequest, "symptom required")
		return
	}

	feedbackDir := filepath.Join(h.DecksDir, "..", "feedback")
	os.MkdirAll(feedbackDir, 0755)

	entry := map[string]any{
		"type":       body.Type,
		"page":       body.Page,
		"context":    body.Context,
		"symptom":    body.Symptom,
		"expected":   body.Expected,
		"contact":    body.Contact,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"user_agent": r.UserAgent(),
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	fname := fmt.Sprintf("%d-%s.json", time.Now().UnixMilli(), body.Type)
	if err := os.WriteFile(filepath.Join(feedbackDir, fname), data, 0644); err != nil {
		log.Printf("feedback write error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("feedback received: type=%s page=%s contact=%s", body.Type, body.Page, body.Contact)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleKofiWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	dataStr := r.FormValue("data")
	if dataStr == "" {
		writeError(w, http.StatusBadRequest, "missing data")
		return
	}

	var payload struct {
		VerificationToken        string  `json:"verification_token"`
		MessageID                string  `json:"message_id"`
		Timestamp                string  `json:"timestamp"`
		Type                     string  `json:"type"`
		IsPublic                 bool    `json:"is_public"`
		FromName                 string  `json:"from_name"`
		Message                  string  `json:"message"`
		Amount                   string  `json:"amount"`
		URL                      string  `json:"url"`
		Email                    string  `json:"email"`
		Currency                 string  `json:"currency"`
		IsSubscriptionPayment    bool    `json:"is_subscription_payment"`
		IsFirstSubscriptionPayment bool  `json:"is_first_subscription_payment"`
		TierName                 *string `json:"tier_name"`
	}
	if err := json.Unmarshal([]byte(dataStr), &payload); err != nil {
		log.Printf("kofi webhook: bad JSON: %v", err)
		writeError(w, http.StatusBadRequest, "bad data")
		return
	}

	expectedToken := os.Getenv("KOFI_VERIFICATION_TOKEN")
	if expectedToken != "" && payload.VerificationToken != expectedToken {
		log.Printf("kofi webhook: token mismatch")
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	donationsDir := filepath.Join(h.DecksDir, "..", "donations")
	os.MkdirAll(donationsDir, 0755)

	entry := map[string]any{
		"message_id":    payload.MessageID,
		"timestamp":     payload.Timestamp,
		"type":          payload.Type,
		"is_public":     payload.IsPublic,
		"from_name":     payload.FromName,
		"message":       payload.Message,
		"amount":        payload.Amount,
		"currency":      payload.Currency,
		"is_subscription": payload.IsSubscriptionPayment,
		"tier_name":     payload.TierName,
		"received_at":   time.Now().UTC().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(entry, "", "  ")
	fname := fmt.Sprintf("%d-%s.json", time.Now().UnixMilli(), payload.MessageID)
	if err := os.WriteFile(filepath.Join(donationsDir, fname), data, 0644); err != nil {
		log.Printf("kofi write error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	log.Printf("kofi donation: %s %s from %s (type=%s, public=%v)", payload.Amount, payload.Currency, payload.FromName, payload.Type, payload.IsPublic)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) handleDonationsSummary(w http.ResponseWriter, r *http.Request) {
	donationsDir := filepath.Join(h.DecksDir, "..", "donations")
	entries, err := os.ReadDir(donationsDir)
	if err != nil {
		writeJSON(w, map[string]any{"month_total": 0, "all_time_total": 0, "recent": []any{}})
		return
	}

	type donation struct {
		FromName  string `json:"from_name"`
		Amount    string `json:"amount"`
		Message   string `json:"message,omitempty"`
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var monthTotal, allTimeTotal float64
	var recent []donation

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(donationsDir, e.Name()))
		if err != nil {
			continue
		}
		var d struct {
			FromName  string `json:"from_name"`
			Amount    string `json:"amount"`
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
			IsPublic  bool   `json:"is_public"`
			Type      string `json:"type"`
		}
		if json.Unmarshal(raw, &d) != nil {
			continue
		}

		var amt float64
		fmt.Sscanf(d.Amount, "%f", &amt)
		allTimeTotal += amt

		ts, _ := time.Parse(time.RFC3339, d.Timestamp)
		if ts.IsZero() {
			ts, _ = time.Parse("2006-01-02T15:04:05Z", d.Timestamp)
		}
		if !ts.Before(monthStart) {
			monthTotal += amt
		}

		if d.IsPublic {
			name := d.FromName
			recent = append(recent, donation{
				FromName:  name,
				Amount:    d.Amount,
				Message:   d.Message,
				Timestamp: d.Timestamp,
				Type:      d.Type,
			})
		}
	}

	sort.Slice(recent, func(i, j int) bool { return recent[i].Timestamp > recent[j].Timestamp })
	if len(recent) > 10 {
		recent = recent[:10]
	}

	writeJSON(w, map[string]any{
		"month_total":    monthTotal,
		"all_time_total": allTimeTotal,
		"month_goal":     202,
		"recent":         recent,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}
