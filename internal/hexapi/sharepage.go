package hexapi

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// IndexHTMLPath is the path to the built SPA index.html (e.g. hexdek/dist/
// index.html on the static host). When set on the Handler, deck share pages
// served from this Go server will be a copy of that file with the OG_META
// block rewritten to deck-specific values for crawler previews. When empty
// or the file is missing, the handler returns a minimal HTML stub with just
// the OG tags + a meta-refresh redirect to the canonical SPA URL — still
// good enough for Discord/Twitter unfurls.
//
// In production the SPA HTML is served by Caddy from the static host. To
// enable rich crawler previews, point Caddy at this Go endpoint either
// unconditionally for /decks/{owner}/{id} or only for crawler User-Agents.

const (
	ogMetaStartMarker = "<!-- OG_META_START -->"
	ogMetaEndMarker   = "<!-- OG_META_END -->"

	publicSiteOrigin = "https://hexdek.dev"
)

func (h *Handler) handleDeckSharePage(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	cmdrName, summary, ok := h.loadDeckShareMeta(owner, id)
	if !ok {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	// Title: prefer the commander's printed name (cmdrName) over the
	// derived deck name; if missing, fall back to the deck id.
	title := cmdrName
	if title == "" {
		title = strings.ToUpper(strings.ReplaceAll(id, "_", " "))
	}
	if summary == "" {
		summary = fmt.Sprintf("%s — Commander deck on HEXDEK.", title)
	}

	pageURL := fmt.Sprintf("%s/decks/%s/%s", publicSiteOrigin, owner, id)
	imageURL := pageURL // safe default — overridden below if we have a commander
	if cmdrName != "" {
		imageURL = fmt.Sprintf("%s/api/card-art/%s", publicSiteOrigin, url.PathEscape(strings.Split(cmdrName, "//")[0]))
	}

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

	// Fallback: minimal HTML with OG tags + a meta-refresh into the SPA so
	// human visitors who happen to land on this Go endpoint still get there.
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

// loadDeckShareMeta returns the commander card name and a one-line gameplan
// summary for OG previews. Falls back gracefully when Freya analysis is
// missing — the share page should still render with whatever we have.
func (h *Handler) loadDeckShareMeta(owner, id string) (cmdrName, summary string, ok bool) {
	deckPath := findDeckFile(h.DecksDir, owner, id)
	if deckPath == "" {
		return "", "", false
	}
	cmdrName = extractCommander(deckPath)

	strategyFile := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	if data, err := os.ReadFile(strategyFile); err == nil {
		var strat struct {
			GameplanSummary string `json:"gameplan_summary"`
			Archetype       string `json:"archetype"`
		}
		if err := json.Unmarshal(data, &strat); err == nil {
			summary = strat.GameplanSummary
			if summary == "" && strat.Archetype != "" {
				summary = "Archetype: " + strat.Archetype
			}
		}
	}
	return cmdrName, summary, true
}

func buildOGBlock(title, description, pageURL, imageURL string) string {
	t := html.EscapeString(title)
	d := html.EscapeString(description)
	u := html.EscapeString(pageURL)
	img := html.EscapeString(imageURL)
	var b strings.Builder
	b.WriteString(ogMetaStartMarker)
	b.WriteByte('\n')
	fmt.Fprintf(&b, `<meta property="og:site_name" content="HEXDEK" />`+"\n")
	fmt.Fprintf(&b, `<meta property="og:title" content="%s" />`+"\n", t)
	fmt.Fprintf(&b, `<meta property="og:description" content="%s" />`+"\n", d)
	fmt.Fprintf(&b, `<meta property="og:type" content="article" />`+"\n")
	fmt.Fprintf(&b, `<meta property="og:url" content="%s" />`+"\n", u)
	fmt.Fprintf(&b, `<meta property="og:image" content="%s" />`+"\n", img)
	fmt.Fprintf(&b, `<meta name="twitter:card" content="summary_large_image" />`+"\n")
	fmt.Fprintf(&b, `<meta name="twitter:title" content="%s" />`+"\n", t)
	fmt.Fprintf(&b, `<meta name="twitter:description" content="%s" />`+"\n", d)
	fmt.Fprintf(&b, `<meta name="twitter:image" content="%s" />`+"\n", img)
	b.WriteString(ogMetaEndMarker)
	return b.String()
}

func injectOG(htmlSrc, ogBlock string) (string, bool) {
	startIdx := strings.Index(htmlSrc, ogMetaStartMarker)
	endIdx := strings.Index(htmlSrc, ogMetaEndMarker)
	if startIdx < 0 || endIdx < 0 || endIdx < startIdx {
		return "", false
	}
	endIdx += len(ogMetaEndMarker)
	return htmlSrc[:startIdx] + ogBlock + htmlSrc[endIdx:], true
}
