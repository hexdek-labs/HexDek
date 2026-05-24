package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// handleShareDeckPage serves /share/{owner}/{id} — the round-15 link-
// preview route. OG title is deck name + commander, description is
// archetype · bracket · win rate, image is the commander's card art.
//
// Like the other share endpoints in this package, the body is the
// SPA's index.html with the OG_META block rewritten — so the SPA
// hydrates normally for human visitors while crawler User-Agents
// (Discord, Twitter, Slack, Facebook) parse per-deck OG. Caddy must
// route /share/{owner}/{id} to this backend (mirror of the existing
// /decks/{owner}/{id} Caddy rule).
func (h *Handler) handleShareDeckPage(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	meta, ok := h.loadShareDeckMeta(r.Context(), owner, id)
	if !ok {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	pageURL := fmt.Sprintf("%s/share/%s/%s", publicSiteOrigin, owner, id)
	h.renderSharePage(w,
		buildShareTitle(meta),
		buildShareSummary(meta),
		pageURL,
		buildShareImageURL(meta))
}

type shareDeckMeta struct {
	Owner     string
	ID        string
	DeckName  string  // custom_name override → falls back to slugified id
	Commander string  // printed commander card name
	Archetype string  // freya archetype slug (e.g. "artifacts", "voltron")
	Bracket   int     // 1-5, 0 when unknown
	Games     int
	Wins      int
	WinRate   float64 // 0..100
}

func (h *Handler) loadShareDeckMeta(ctx context.Context, owner, id string) (shareDeckMeta, bool) {
	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		return shareDeckMeta{}, false
	}
	m := shareDeckMeta{Owner: owner, ID: id}
	m.Commander = extractCommander(deckPath)

	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		var custom sql.NullString
		_ = h.Showmatch.sqlDB.QueryRowContext(ctx,
			`SELECT custom_name FROM deck_meta WHERE owner = ? AND id = ?`,
			owner, id).Scan(&custom)
		if custom.Valid {
			m.DeckName = strings.TrimSpace(custom.String)
		}
	}
	if m.DeckName == "" {
		m.DeckName = slugToDeckName(id, owner)
	}

	strategyFile := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	if data, err := os.ReadFile(strategyFile); err == nil {
		var strat struct {
			Archetype string `json:"archetype"`
			Bracket   int    `json:"bracket"`
		}
		if err := json.Unmarshal(data, &strat); err == nil {
			m.Archetype = strat.Archetype
			m.Bracket = strat.Bracket
		}
	}

	if h.Showmatch != nil && h.Showmatch.sqlDB != nil {
		deckKey := owner + "/" + id
		var losses int
		_ = h.Showmatch.sqlDB.QueryRowContext(ctx,
			`SELECT games, wins, losses FROM showmatch_elo WHERE deck_key = ? LIMIT 1`,
			deckKey).Scan(&m.Games, &m.Wins, &losses)
		if m.Games > 0 {
			m.WinRate = float64(m.Wins) / float64(m.Games) * 100
		}
	}
	return m, true
}

// slugToDeckName mirrors SharePreview.jsx slugToTitle so the OG title
// matches what humans see when the SPA renders. Strips trailing
// version hashes (8+ chars), owner suffix, and bracket suffix.
func slugToDeckName(id, owner string) string {
	if id == "" {
		return ""
	}
	s := id
	if i := strings.LastIndex(s, "_"); i >= 0 && len(s)-i-1 >= 8 {
		suffix := s[i+1:]
		isHashLike := true
		for _, c := range suffix {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				isHashLike = false
				break
			}
		}
		if isHashLike {
			s = s[:i]
		}
	}
	if owner != "" {
		s = strings.TrimSuffix(strings.ToLower(s), "_"+strings.ToLower(owner))
	}
	for _, suf := range []string{"_b0", "_b1", "_b2", "_b3", "_b4", "_b5"} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.ToUpper(strings.ReplaceAll(s, "_", " "))
}

func buildShareTitle(m shareDeckMeta) string {
	deckTitle := strings.TrimSpace(m.DeckName)
	if deckTitle == "" {
		deckTitle = m.Commander
	}
	if m.Commander != "" && !strings.EqualFold(deckTitle, m.Commander) {
		return fmt.Sprintf("%s · %s", deckTitle, m.Commander)
	}
	if deckTitle == "" {
		return "HEXDEK Deck"
	}
	return deckTitle
}

func buildShareSummary(m shareDeckMeta) string {
	parts := []string{}
	if m.Archetype != "" {
		parts = append(parts, archetypeLabel(m.Archetype))
	}
	if m.Bracket > 0 {
		parts = append(parts, fmt.Sprintf("Bracket B%d", m.Bracket))
	}
	if m.Games > 0 {
		parts = append(parts, fmt.Sprintf("%d%% WR · %d games",
			int(math.Round(m.WinRate)), m.Games))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%s — Commander deck on HEXDEK.", buildShareTitle(m))
	}
	return strings.Join(parts, " · ")
}

func archetypeLabel(slug string) string {
	if slug == "" {
		return ""
	}
	out := strings.ReplaceAll(slug, "_", " ")
	parts := strings.Fields(out)
	for i, p := range parts {
		if len(p) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func buildShareImageURL(m shareDeckMeta) string {
	if m.Commander == "" {
		return fmt.Sprintf("%s/og-default.png", publicSiteOrigin)
	}
	artName := strings.TrimSpace(strings.Split(m.Commander, "//")[0])
	return fmt.Sprintf("%s/api/card-art/%s", publicSiteOrigin, url.PathEscape(artName))
}
