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

	"github.com/hexdek/hexdek/internal/db"
)

type Handler struct {
	DecksDir  string
	Showmatch *Showmatch
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
}

type DeckSummary struct {
	ID            string    `json:"id"`
	Owner         string    `json:"owner"`
	Name          string    `json:"name"`
	Commander     string    `json:"commander"`
	CommanderCard string    `json:"commander_card,omitempty"`
	CardCount     int       `json:"card_count"`
	Bracket       string    `json:"bracket"`
	Color         string    `json:"color"`
	ImportedAt    time.Time `json:"imported_at"`
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
		if owner == "freya" || owner == "benched" || owner == "test" || owner == "moxfield_300" {
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

	sort.Slice(decks, func(i, j int) bool {
		return decks[i].ImportedAt.After(decks[j].ImportedAt)
	})

	writeJSON(w, decks)
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
	}
	writeJSON(w, map[string]any{
		"id":             id,
		"owner":          owner,
		"commander":      commander,
		"commander_card": cmdrCard,
		"bracket":        bracket,
		"color":          color,
		"card_count":     totalCards,
		"cards":          cards,
	})
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

	freyaBin := "mtgsquad-freya"
	if _, err := exec.LookPath(freyaBin); err != nil {
		freyaBin = "./mtgsquad-freya"
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
	writeJSON(w, map[string]any{
		"id":             finalID,
		"owner":          owner,
		"name":           name,
		"commander_card": cmdrCard,
		"card_count":     len(cards),
		"file_path":      filepath.Join(owner, filepath.Base(deckPath)),
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
		cards = append(cards, map[string]any{
			"name":     c.Name,
			"quantity": c.Quantity,
			"cmc":      c.CMC,
		})
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
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		w.Write(cached.([]byte))
		return
	}

	cachePath := filepath.Join(artCacheDir, hash+".jpg")
	if data, err := os.ReadFile(cachePath); err == nil {
		artMemCache.Store(hash, data)
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
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
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		w.Write(res.data)
		return
	}
	defer artInflight.Delete(hash)

	clean := strings.Split(name, "//")[0]
	clean = strings.TrimSpace(clean)
	scryfallURL := "https://api.scryfall.com/cards/named?fuzzy=" + url.QueryEscape(clean) + "&format=image&version=art_crop"

	req, _ := http.NewRequest("GET", scryfallURL, nil)
	req.Header.Set("User-Agent", "HexDek/1.0 (mtgsquad card art cache)")
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
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.Write(data)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		log.Printf("json encode: %v", err)
	}
}
