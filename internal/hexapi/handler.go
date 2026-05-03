package hexapi

import (
	"crypto/sha256"
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
	"strings"
	"sync"
	"time"

	"github.com/hexdek/hexdek/internal/analytics"
	"github.com/hexdek/hexdek/internal/db"
	"github.com/hexdek/hexdek/internal/versioning"
)

type Handler struct {
	DecksDir  string
	Showmatch *Showmatch
	cardDB    map[string]oracleCard

	deckSubsMu sync.RWMutex
	deckSubs   map[string]map[chan deckEvent]struct{}
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
		}
	}
	log.Printf("carddb: loaded %d cards", len(h.cardDB))
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/decks", h.handleListDecks)
	mux.HandleFunc("POST /api/decks", h.handleImportDeck)
	mux.HandleFunc("GET /api/decks/{owner}/{id}", h.handleGetDeck)
	mux.HandleFunc("PUT /api/decks/{owner}/{id}", h.handleUpdateDeck)
	mux.HandleFunc("DELETE /api/decks/{owner}/{id}", h.handleDeleteDeck)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/versions", h.handleListVersions)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/analysis", h.handleGetAnalysis)
	mux.HandleFunc("POST /api/decks/{owner}/{id}/analyze", h.handleRunAnalysis)
	mux.HandleFunc("GET /api/profile", h.handleProfile)
	mux.HandleFunc("GET /api/card-art/{name}", h.handleCardArt)
	mux.HandleFunc("GET /api/card-stats/{commander}", h.handleCardWinStats)
	mux.HandleFunc("GET /api/rivalry/{owner}/{id}", h.handleRivalry)
	mux.HandleFunc("GET /api/threat-graph/{owner}/{id}", h.handleThreatGraph)
	mux.HandleFunc("GET /api/leaderboard", h.handleLeaderboard)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/lineage", h.handleDeckLineage)
	mux.HandleFunc("POST /api/import/moxfield", h.handleMoxfieldImport)
	mux.HandleFunc("GET /api/decks/{owner}/{id}/events", h.handleDeckEvents)
	mux.HandleFunc("POST /api/feedback", h.handleFeedback)
	mux.HandleFunc("POST /api/kofi/webhook", h.handleKofiWebhook)
	mux.HandleFunc("GET /api/donations/summary", h.handleDonationsSummary)
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
}

func (h *Handler) handleListDecks(w http.ResponseWriter, r *http.Request) {
	ownerFilter := r.URL.Query().Get("owner")

	var decks []DeckSummary
	owners, err := os.ReadDir(h.DecksDir)
	if err != nil {
		http.Error(w, "cannot read decks directory", http.StatusInternalServerError)
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
			commander, bracket, color := parseDeckFilename(name)

			fullPath := filepath.Join(deckDir, f.Name())
			cards := countCards(fullPath)
			cmdrCard := extractCommander(fullPath)

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
}

func (h *Handler) handleGetDeck(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	data, err := os.ReadFile(deckPath)
	if err != nil {
		http.Error(w, "cannot read deck", http.StatusInternalServerError)
		return
	}

	var cards []map[string]any
	if strings.HasSuffix(deckPath, ".json") {
		cards = parseDeckJSON(data)
	} else {
		cards = parseDeckList(string(data))
	}
	commander, bracket, color := parseDeckFilename(id)
	cmdrCard := extractCommander(deckPath)

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
	writeJSON(w, map[string]any{
		"id":              id,
		"owner":           owner,
		"commander":       commander,
		"commander_card":  cmdrCard,
		"bracket":         bracket,
		"color":           color,
		"card_count":      totalCards,
		"cards":           cards,
		"mana_production": production,
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
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		DeckList string `json:"deck_list"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.DeckList) == "" {
		http.Error(w, "deck_list is required", http.StatusBadRequest)
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
		http.Error(w, "cannot write deck file", http.StatusInternalServerError)
		return
	}

	cards := parseDeckList(req.DeckList)
	cmdrCard := extractCommander(deckPath)
	commander, bracket, color := parseDeckFilename(id)

	// Register new version in the DAG with prior inheritance.
	var cardNames []string
	for _, c := range cards {
		if n, ok := c["name"].(string); ok {
			cardNames = append(cardNames, n)
		}
	}
	go h.registerDeckVersion(owner, id, cmdrCard, cardNames)

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
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	if err := os.Remove(deckPath); err != nil {
		http.Error(w, "cannot delete deck", http.StatusInternalServerError)
		return
	}

	// Clean up Freya analysis if it exists
	strategyFile := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	os.Remove(strategyFile)

	writeJSON(w, map[string]any{"deleted": true, "id": id, "owner": owner})
}

func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
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

func (h *Handler) handleGetAnalysis(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
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
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (h *Handler) handleRunAnalysis(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
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
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
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
	profile := map[string]any{
		"username":     "OPERATOR",
		"userId":       "USR.0001",
		"joined":       "04.2026",
		"elo":          1500,
		"eloChange":    0,
		"tier":         "UNRANKED",
		"streak":       "—",
		"primaryColor": "—",
		"archetype":    "OBSERVER",
		"percentile":   "—",
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
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	DeckList string `json:"deck_list"`
}

func (h *Handler) handleImportDeck(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.DeckList) == "" {
		http.Error(w, "deck_list is required", http.StatusBadRequest)
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
		http.Error(w, "cannot create deck directory", http.StatusInternalServerError)
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
			http.Error(w, "too many decks with the same name", http.StatusConflict)
			return
		}
	}

	if err := os.WriteFile(deckPath, []byte(req.DeckList), 0644); err != nil {
		http.Error(w, "cannot write deck file", http.StatusInternalServerError)
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

	writeJSON(w, map[string]any{
		"id":             finalID,
		"owner":          owner,
		"name":           name,
		"commander_card": cmdrCard,
		"card_count":     len(cards),
		"file_path":      filepath.Join(owner, filepath.Base(deckPath)),
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
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		URL   string `json:"url"`
		Owner string `json:"owner"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Host != "www.moxfield.com" {
		http.Error(w, "invalid Moxfield URL", http.StatusBadRequest)
		return
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "decks" {
		http.Error(w, "URL must be https://www.moxfield.com/decks/{id}", http.StatusBadRequest)
		return
	}
	moxID := parts[1]

	apiURL := "https://api2.moxfield.com/v3/decks/all/" + url.PathEscape(moxID)
	apiReq, _ := http.NewRequest("GET", apiURL, nil)
	apiReq.Header.Set("User-Agent", "HexDek/1.0 (hexdek deck import)")
	apiReq.Header.Set("Accept", "application/json")
	resp, err := moxfieldClient.Do(apiReq)
	if err != nil {
		writeJSON(w, map[string]string{"error": "failed to fetch from Moxfield: " + err.Error()})
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		writeJSON(w, map[string]string{"error": fmt.Sprintf("Moxfield returned %d", resp.StatusCode)})
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		http.Error(w, "failed to read Moxfield response", http.StatusBadGateway)
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
		http.Error(w, "failed to parse Moxfield response", http.StatusBadGateway)
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
		http.Error(w, "cannot create deck directory", http.StatusInternalServerError)
		return
	}

	deckPath := filepath.Join(ownerDir, fileID+".txt")
	for i := 2; ; i++ {
		if _, err := os.Stat(deckPath); os.IsNotExist(err) {
			break
		}
		deckPath = filepath.Join(ownerDir, fmt.Sprintf("%s_%d.txt", fileID, i))
		if i > 100 {
			http.Error(w, "too many decks with the same name", http.StatusConflict)
			return
		}
	}

	if err := os.WriteFile(deckPath, []byte(deckList), 0644); err != nil {
		http.Error(w, "cannot write deck file", http.StatusInternalServerError)
		return
	}

	finalID := strings.TrimSuffix(filepath.Base(deckPath), ".txt")
	go h.registerDeckVersion(owner, finalID, cmdrName, cardNames)

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{
		"id":         finalID,
		"owner":      owner,
		"name":       deckName,
		"commander":  cmdrName,
		"card_count": len(cardNames),
		"source":     "moxfield",
		"moxfield_id": moxID,
	})
}

func (h *Handler) handleCardWinStats(w http.ResponseWriter, r *http.Request) {
	commander := r.PathValue("commander")
	if commander == "" {
		http.Error(w, "commander required", http.StatusBadRequest)
		return
	}
	commander = strings.ReplaceAll(commander, "_", " ")
	if h.Showmatch == nil || h.Showmatch.sqlDB == nil {
		writeJSON(w, map[string]any{"cards": []any{}, "commander": commander})
		return
	}
	stats, err := db.LoadCardWinStats(r.Context(), h.Showmatch.sqlDB, commander, 50)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"commander": commander,
		"cards":     stats,
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
		http.Error(w, "missing card name", http.StatusBadRequest)
		return
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(name))))

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
			http.Error(w, "card art not found", http.StatusNotFound)
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
	scryfallURL := "https://api.scryfall.com/cards/named?fuzzy=" + url.QueryEscape(clean) + "&format=image&version=art_crop"

	req, _ := http.NewRequest("GET", scryfallURL, nil)
	req.Header.Set("User-Agent", "HexDek/1.0 (hexdek card art cache)")
	req.Header.Set("Accept", "image/*")
	resp, err := artHTTPClient.Do(req)
	if err != nil {
		ch <- artResult{err: err}
		http.Error(w, "card art not found", http.StatusNotFound)
		return
	}
	if resp.StatusCode != 200 {
		resp.Body.Close()
		ch <- artResult{err: fmt.Errorf("scryfall %d", resp.StatusCode)}
		http.Error(w, "card art not found", http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		ch <- artResult{err: err}
		http.Error(w, "failed to read art", http.StatusInternalServerError)
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
		http.Error(w, "cannot load version DAG", http.StatusInternalServerError)
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
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	dag, err := versioning.LoadDAG(filepath.Join(h.DecksDir, ".versions"))
	if err != nil {
		http.Error(w, "cannot load version DAG", http.StatusInternalServerError)
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
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	commander := extractCommander(deckPath)
	if commander == "" {
		commander, _, _ = parseDeckFilename(id)
	}

	rivalries, err := analytics.LoadRivalries("data/rivalry")
	if err != nil {
		http.Error(w, "cannot load rivalry data", http.StatusInternalServerError)
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
		http.Error(w, "invalid owner or id", http.StatusBadRequest)
		return
	}

	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		http.Error(w, "deck not found", http.StatusNotFound)
		return
	}

	commander := extractCommander(deckPath)
	if commander == "" {
		commander, _, _ = parseDeckFilename(id)
	}

	edges, err := analytics.LoadThreatGraph("data/analytics")
	if err != nil {
		http.Error(w, "cannot load threat graph", http.StatusInternalServerError)
		return
	}

	summary := analytics.ThreatSummaryFor(edges, commander, 10)
	writeJSON(w, summary)
}

func (h *Handler) handleFeedback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type     string `json:"type"`
		Page     string `json:"page"`
		Context  string `json:"context"`
		Symptom  string `json:"symptom"`
		Expected string `json:"expected"`
		Contact  string `json:"contact"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 32768)).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if body.Symptom == "" {
		http.Error(w, "symptom required", http.StatusBadRequest)
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
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("feedback received: type=%s page=%s contact=%s", body.Type, body.Page, body.Contact)
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleKofiWebhook(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	dataStr := r.FormValue("data")
	if dataStr == "" {
		http.Error(w, "missing data", http.StatusBadRequest)
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
		http.Error(w, "bad data", http.StatusBadRequest)
		return
	}

	expectedToken := os.Getenv("KOFI_VERIFICATION_TOKEN")
	if expectedToken != "" && payload.VerificationToken != expectedToken {
		log.Printf("kofi webhook: token mismatch")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
		http.Error(w, "internal error", http.StatusInternalServerError)
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
