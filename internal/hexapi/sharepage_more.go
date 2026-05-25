package hexapi

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hexdek/hexdek/internal/versioning"
)

// renderSharePage writes an HTML response with the given OG block injected
// into the SPA's index.html (when IndexHTMLPath is configured) or a
// minimal stub with a meta-refresh into the canonical SPA URL. Mirrors
// the rendering tail of handleDeckSharePage so each share endpoint can
// share the same crawler-vs-browser fallback behavior.
func (h *Handler) renderSharePage(w http.ResponseWriter, title, summary, pageURL, imageURL string) {
	og := buildOGBlock(title, summary, pageURL, imageURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")

	if h.IndexHTMLPath != "" {
		raw, err := os.ReadFile(h.IndexHTMLPath)
		if err == nil {
			if injected, ok := injectOG(string(raw), og); ok {
				w.Write([]byte(injected))
				return
			}
		}
	}

	stub := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s — HEXDEK</title>
%s
<meta http-equiv="refresh" content="0; url=%s">
</head>
<body><a href="%s">%s</a></body>
</html>`, html.EscapeString(title), og, html.EscapeString(pageURL), html.EscapeString(pageURL), html.EscapeString(title))
	w.Write([]byte(stub))
}

// handleCardSharePage serves /cards/{name} with card-specific OG meta.
// Description is derived from the card's oracle text (truncated); the
// image is the Scryfall art_crop served through our /api/card-art proxy.
func (h *Handler) handleCardSharePage(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("name")
	name, err := url.PathUnescape(raw)
	if err != nil || name == "" {
		writeError(w, http.StatusBadRequest, "invalid name")
		return
	}

	display := name
	summary := ""
	if info, ok := h.cardDB[strings.ToLower(strings.TrimSpace(name))]; ok {
		display = titleCaseCardName(strings.ToLower(strings.TrimSpace(name)))
		summary = strings.TrimSpace(info.OracleText)
		if summary == "" {
			summary = strings.TrimSpace(info.TypeLine)
		}
		summary = collapseWhitespace(summary)
	}
	if summary == "" {
		summary = fmt.Sprintf("%s — Magic: The Gathering card on HEXDEK.", display)
	}
	summary = truncateOG(summary, 280)

	pageURL := fmt.Sprintf("%s/cards/%s", publicSiteOrigin, url.PathEscape(display))
	// Use just the front face for art lookup (DFCs use "Front // Back").
	artName := strings.TrimSpace(strings.Split(display, "//")[0])
	imageURL := fmt.Sprintf("%s/api/card-art/%s", publicSiteOrigin, url.PathEscape(artName))

	h.renderSharePage(w, display, summary, pageURL, imageURL)
}

// handleOperatorSharePage serves /operator/{owner} with the owner's top
// deck and best ELO surfaced in the OG meta.
func (h *Handler) handleOperatorSharePage(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if !validatePathComponent(owner) {
		writeError(w, http.StatusBadRequest, "invalid owner")
		return
	}
	ownerLower := strings.ToLower(owner)

	var topCommander string
	var topRating float64
	var totalGames int
	if dag, err := versioning.LoadDAG(filepath.Join(h.DecksDir, ".versions")); err == nil {
		for _, head := range dag.Leaderboard() {
			if strings.ToLower(head.Owner) != ownerLower {
				continue
			}
			totalGames += head.GamesPlayed
			r := head.Rating.Conservative()
			if topCommander == "" || r > topRating {
				topRating = r
				topCommander = head.Commander
			}
		}
	}

	title := strings.ToUpper(owner)
	if topRating > 0 {
		title = fmt.Sprintf("%s · %d ELO", strings.ToUpper(owner), int(math.Round(topRating)))
	}

	var summary string
	switch {
	case topCommander != "" && totalGames > 0:
		summary = fmt.Sprintf("%s pilots %s on HEXDEK · %d games logged.", owner, topCommander, totalGames)
	case topCommander != "":
		summary = fmt.Sprintf("%s pilots %s on HEXDEK.", owner, topCommander)
	default:
		summary = fmt.Sprintf("%s — operator profile on HEXDEK.", owner)
	}

	pageURL := fmt.Sprintf("%s/operator/%s", publicSiteOrigin, url.PathEscape(owner))
	imageURL := fmt.Sprintf("%s/og-default.png", publicSiteOrigin)
	if topCommander != "" {
		artName := strings.TrimSpace(strings.Split(topCommander, "//")[0])
		imageURL = fmt.Sprintf("%s/api/card-art/%s", publicSiteOrigin, url.PathEscape(artName))
	}

	h.renderSharePage(w, title, summary, pageURL, imageURL)
}

// handleSpectateSharePage serves /spectate with static "HEXDEK Live" OG
// meta. The live game state isn't snapshotted into the share preview
// because crawlers cache aggressively and a stale game would mislead.
func (h *Handler) handleSpectateSharePage(w http.ResponseWriter, r *http.Request) {
	pageURL := fmt.Sprintf("%s/spectate", publicSiteOrigin)
	imageURL := fmt.Sprintf("%s/og-default.png", publicSiteOrigin)
	h.renderSharePage(w,
		"HEXDEK Live",
		"Watch four AIs pilot Commander decks in real time on HEXDEK.",
		pageURL, imageURL)
}

// handleLeaderboardSharePage serves /leaderboard with the global ELO
// ladder framing.
func (h *Handler) handleLeaderboardSharePage(w http.ResponseWriter, r *http.Request) {
	pageURL := fmt.Sprintf("%s/leaderboard", publicSiteOrigin)
	imageURL := fmt.Sprintf("%s/og-default.png", publicSiteOrigin)
	h.renderSharePage(w,
		"HEXDEK Leaderboard",
		"Live ELO standings for every deck logged into the HEXDEK engine.",
		pageURL, imageURL)
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateOG(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndex(cut, " "); i > max-40 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:.") + "…"
}
