package hexapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hexdek/hexdek/internal/deckparser"
	"github.com/hexdek/hexdek/internal/gameengine"
)

// deck_import_endpoint_r60.go — POST /api/deck/import is the
// "preview / dry-run" surface complementing the existing POST /api/decks
// (which persists). Takes deck text in any format deckparser supports
// (Commander w/ COMMANDER: + Library, Brawl, Oathbreaker, MTGO export,
// Aetherhub, Archidekt, Deckbox, MTGGoldfish, MTGTop8, TappedOut,
// TCGPlayer — full surface per PRs #835 / #845 / #946 — plus inline
// comments, DFC // splits, leading-bracket prefixes, type-category
// section headers, emoji-stripped names, MTGO metadata blocks) and
// returns:
//
//   {
//     "deck": {
//       "commander":         "Atraxa, Praetors' Voice",
//       "commander_partners": ["..."],
//       "format":            "commander",
//       "card_count":        100,
//       "lines":             [{qty, name, section, status, line_number, comment}],
//       "unresolved":        [...],
//       "parse_coverage":    98.5
//     },
//     "quick_analyze": {
//       "lands":          37,
//       "creatures":      24,
//       "instants":       8,
//       "sorceries":      6,
//       "enchantments":   5,
//       "artifacts":      9,
//       "planeswalkers":  2,
//       "battles":        0,
//       "other":          1,
//       "average_cmc":    3.1,
//       "color_identity": ["W", "U", "B", "G"],
//       "sideboard":      0,
//       "signature_spells": 0,
//       "warnings":       ["..."]
//     }
//   }
//
// This is a PURE PARSE — no disk write, no DB row. Use POST /api/decks
// when you want to persist.
//
// Auth: X-HexDek-Owner header required (matches the existing
// authenticated-import shape in hexapi). 401 when missing. The owner
// slug is NOT used for filesystem routing here — the endpoint never
// touches DecksDir — but the requirement keeps the rate-limit per-
// owner bucket meaningful and provides a hook for future per-owner
// preview history.
//
// Rate limit: dual-axis (per-IP shared bucket with the other import
// endpoints + per-owner). Reuses h.DeckImportLimiter +
// h.DeckAnalyzeOwnerLimiter — sharing those buckets means a session
// that's burning quota on POST /api/decks can't bypass it via
// POST /api/deck/import for the same effective work.
//
// No CSRF: the endpoint is idempotent (pure parse, no persisted
// side-effect). Adding CSRF would force the caller to fetch a token
// for an effectively read-only operation, which is friction without
// security value. The endpoint is also not on the existing
// csrfGatedRoutes pin so the coverage test stays green.

// DeckImportPreviewRequest is the JSON body shape for POST /api/deck/import.
type DeckImportPreviewRequest struct {
	DeckList string `json:"deck_list"`
}

// DeckImportPreviewResponse is the response envelope.
type DeckImportPreviewResponse struct {
	Deck         ImportedDeckSummary `json:"deck"`
	QuickAnalyze QuickAnalyze        `json:"quick_analyze"`
}

// ImportedDeckSummary is the parsed-deck side of the response.
type ImportedDeckSummary struct {
	Commander         string             `json:"commander"`
	CommanderPartners []string           `json:"commander_partners"`
	Format            string             `json:"format"`
	CardCount         int                `json:"card_count"`
	Lines             []ImportedCardLine `json:"lines"`
	Unresolved        []string           `json:"unresolved"`
	ParseCoverage     float64            `json:"parse_coverage"`
}

// ImportedCardLine mirrors deckparser.CardLine on the wire.
type ImportedCardLine struct {
	Qty        int    `json:"qty"`
	Name       string `json:"name"`
	Section    string `json:"section"`
	Status     string `json:"status"`
	LineNumber int    `json:"line_number"`
	Comment    string `json:"comment,omitempty"`
}

// QuickAnalyze is the cheap freya-style summary the preview endpoint
// computes inline from the parsed deck. Avoids spawning the heavy
// Freya subprocess for a feature whose primary user (the deck-paste
// UI) wants sub-100ms feedback.
type QuickAnalyze struct {
	Lands           int      `json:"lands"`
	Creatures       int      `json:"creatures"`
	Instants        int      `json:"instants"`
	Sorceries       int      `json:"sorceries"`
	Enchantments    int      `json:"enchantments"`
	Artifacts       int      `json:"artifacts"`
	Planeswalkers   int      `json:"planeswalkers"`
	Battles         int      `json:"battles"`
	Other           int      `json:"other"`
	AverageCMC      float64  `json:"average_cmc"`
	ColorIdentity   []string `json:"color_identity"`
	Sideboard       int      `json:"sideboard"`
	SignatureSpells int      `json:"signature_spells"`
	Warnings        []string `json:"warnings"`
}

// maxDeckImportBytes caps the body size at 1 MiB — matches the same
// cap on POST /api/decks (handler.go:1427). Plenty for any real deck
// text (the largest published precon list is ~6 KiB); tight enough
// to make a paste-DoS expensive.
const maxDeckImportBytes = 1 << 20

// handleDeckImportPreview is the POST /api/deck/import entry point.
func (h *Handler) handleDeckImportPreview(w http.ResponseWriter, r *http.Request) {
	// Rate-limit gate FIRST (cheap reject before parse). Reuses the
	// DeckImportLimiter + DeckAnalyzeOwnerLimiter buckets so this
	// endpoint shares quota with the other import + analyze paths
	// per-IP and per-owner respectively.
	if enforceRateLimitDual(h.DeckImportLimiter, h.DeckAnalyzeOwnerLimiter, w, r, "deck preview") {
		return
	}

	// Auth gate: owner header required.
	caller := strings.TrimSpace(strings.ToLower(r.Header.Get("X-HexDek-Owner")))
	if caller == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parser configuration: needs the Showmatch's corpus + meta. When
	// the Showmatch isn't wired (test/dev binary that didn't construct
	// one) OR the background load hasn't finished, surface a clear
	// 503 so the UI can show a "warming up" state rather than a
	// generic error.
	if h.Showmatch == nil {
		writeError(w, http.StatusServiceUnavailable, "deck parser not configured")
		return
	}
	corpus := h.Showmatch.DeckParserCorpus()
	meta := h.Showmatch.DeckParserMeta()
	if meta == nil {
		writeError(w, http.StatusServiceUnavailable, "deck parser not configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxDeckImportBytes)
	var req DeckImportPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.DeckList) == "" {
		writeError(w, http.StatusBadRequest, "deck_list is required")
		return
	}

	td, err := deckparser.ParseDeckReader(strings.NewReader(req.DeckList), corpus, meta)
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse: "+err.Error())
		return
	}

	writeJSON(w, DeckImportPreviewResponse{
		Deck:         summarizeImportedDeck(td),
		QuickAnalyze: quickAnalyzeDeck(td),
	})
}

// summarizeImportedDeck condenses a parsed TournamentDeck into the
// wire shape. Commander partners stripped from the headline
// "commander" field so single-commander decks don't render with a
// confusing "+" separator.
func summarizeImportedDeck(td *deckparser.TournamentDeck) ImportedDeckSummary {
	if td == nil {
		return ImportedDeckSummary{Lines: []ImportedCardLine{}, Unresolved: []string{}}
	}
	cmdrNames := td.CommanderNames()
	commander := ""
	partners := []string{}
	if len(cmdrNames) > 0 {
		commander = cmdrNames[0]
		if len(cmdrNames) > 1 {
			partners = append(partners, cmdrNames[1:]...)
		}
	}
	lines := make([]ImportedCardLine, 0, len(td.CardLines))
	for _, cl := range td.CardLines {
		lines = append(lines, ImportedCardLine{
			Qty:        cl.Qty,
			Name:       cl.Name,
			Section:    cl.Section,
			Status:     cl.Status.String(),
			LineNumber: cl.LineNumber,
			Comment:    cl.Comment,
		})
	}
	unresolved := make([]string, 0, len(td.Unresolved))
	unresolved = append(unresolved, td.Unresolved...)
	cardCount := len(td.Library)
	if len(td.CommanderCards) > 0 {
		cardCount += len(td.CommanderCards)
	}
	return ImportedDeckSummary{
		Commander:         commander,
		CommanderPartners: partners,
		Format:            string(td.DetectedFormat),
		CardCount:         cardCount,
		Lines:             lines,
		Unresolved:        unresolved,
		ParseCoverage:     td.ParseReport.CoveragePercent(),
	}
}

// quickAnalyzeDeck composes the cheap inline analysis: type tallies,
// average CMC, color identity, sideboard / signature-spell counts,
// and a small warnings list flagging deckbuilder issues that don't
// need a full Freya pass (no commander, low coverage, no lands).
//
// The intent is a UI affordance — "you pasted X, here's what
// HexDek understood" — NOT a competitive analysis. Full Freya runs
// against /api/decks/{owner}/{id}/analyze after persistence.
func quickAnalyzeDeck(td *deckparser.TournamentDeck) QuickAnalyze {
	out := QuickAnalyze{
		ColorIdentity: []string{},
		Warnings:      []string{},
	}
	if td == nil {
		return out
	}
	out.Sideboard = td.SideboardCount
	out.SignatureSpells = td.SignatureSpellCount

	cards := make([]*gameengine.Card, 0, len(td.Library)+len(td.CommanderCards))
	cards = append(cards, td.Library...)
	cards = append(cards, td.CommanderCards...)

	colorSet := map[string]bool{}
	var cmcSum float64
	var cmcSamples int

	for _, c := range cards {
		if c == nil {
			continue
		}
		switch {
		case hasCardType(c, "land"):
			out.Lands++
		case hasCardType(c, "creature"):
			out.Creatures++
		case hasCardType(c, "planeswalker"):
			out.Planeswalkers++
		case hasCardType(c, "instant"):
			out.Instants++
		case hasCardType(c, "sorcery"):
			out.Sorceries++
		case hasCardType(c, "enchantment"):
			out.Enchantments++
		case hasCardType(c, "artifact"):
			out.Artifacts++
		case hasCardType(c, "battle"):
			out.Battles++
		default:
			out.Other++
		}
		// Lands don't contribute to average CMC — they're free to
		// play. Mirrors the convention in cmd/hexdek-freya.
		if !hasCardType(c, "land") {
			cmcSum += float64(c.CMC)
			cmcSamples++
		}
		for _, col := range c.Colors {
			colorSet[strings.ToUpper(strings.TrimSpace(col))] = true
		}
	}

	if cmcSamples > 0 {
		out.AverageCMC = cmcSum / float64(cmcSamples)
	}

	// Canonical WUBRG ordering so output is stable.
	for _, col := range []string{"W", "U", "B", "R", "G"} {
		if colorSet[col] {
			out.ColorIdentity = append(out.ColorIdentity, col)
		}
	}

	if td.CommanderName == "" && len(td.CommanderCards) == 0 {
		out.Warnings = append(out.Warnings, "no commander detected — add a COMMANDER: line if this is a Commander deck")
	}
	if out.Lands == 0 {
		out.Warnings = append(out.Warnings, "no lands resolved — every deck needs a mana base")
	}
	if td.ParseReport.CoveragePercent() < 90 && td.ParseReport.TotalLines > 0 {
		out.Warnings = append(out.Warnings,
			"low parse coverage — check the unresolved list for typos / renamed cards")
	}
	return out
}

// hasCardType returns true if the card's Types slice contains the
// given lowercase type token. Deckparser sets Types from oracle
// type_line and folds to lowercase, so the check is direct.
func hasCardType(c *gameengine.Card, t string) bool {
	if c == nil {
		return false
	}
	for _, ct := range c.Types {
		if ct == t {
			return true
		}
	}
	return false
}
