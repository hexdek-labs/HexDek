package hexapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/versioning"
)

// FreyaExportSchemaVersion is the contract version external
// consumers can pin against. Bump on breaking JSON-shape changes;
// additive fields are non-breaking. Documented at
// https://hexdek.dev/docs/freya-export (when that page lands).
const FreyaExportSchemaVersion = "1.0"

// FreyaExportEnvelope is the OUTER wrapper external integrators
// see. The inner Analysis payload is the same shape Freya writes
// to strategy.json — we don't re-marshal it field-by-field
// because (a) strategy.json's schema is itself the contract Freya
// owns (PR #291 / #292 / #341 etc), and (b) re-marshaling drops
// new Freya fields the next time Freya adds one. The envelope
// adds the metadata external tools want WITHOUT a second API
// call: deck identity, commander, color identity, the R60 extras
// (system_tags from #416, user feedback from #421, version_hash
// from #429), and provenance fields (schema_version, generated_at,
// source).
type FreyaExportEnvelope struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Source        string             `json:"source"`
	Deck          FreyaExportDeck    `json:"deck"`
	Analysis      json.RawMessage    `json:"analysis"`
	Extras        FreyaExportExtras  `json:"extras"`
}

// FreyaExportDeck is the deck context external tools need to make
// sense of the analysis without a second /api/decks/{owner}/{id}
// roundtrip. Card list is the parsed [{name, quantity}, ...]
// shape so consumers don't have to re-tokenize the raw text.
type FreyaExportDeck struct {
	Owner         string           `json:"owner"`
	ID            string           `json:"id"`
	Commander     string           `json:"commander,omitempty"`
	CommanderCard string           `json:"commander_card,omitempty"`
	ColorIdentity string           `json:"color_identity,omitempty"`
	CardCount     int              `json:"card_count"`
	Cards         []map[string]any `json:"cards"`
}

// FreyaExportExtras bundles the R60-era signals that live outside
// strategy.json but should travel with an export. UserArchetypeFeedback
// is omitempty so consumers can JSON-distinguish "no opinion"
// (absent) from "" (empty string).
type FreyaExportExtras struct {
	SystemTags             []string `json:"system_tags,omitempty"`
	UserArchetypeFeedback  string   `json:"user_archetype_feedback,omitempty"`
	VersionHash            string   `json:"version_hash,omitempty"`
	VersionNumber          int      `json:"version_number,omitempty"`
}

// handleFreyaExport serves GET /api/decks/{owner}/{id}/freya-export.
// Public read — Freya analysis is part of the deck's public face
// (same as /api/decks/{owner}/{id}/analysis), wrapping it for
// external integrators doesn't change the privacy surface.
//
// Distinct from /analysis in three ways:
//
//  1. Returns the deck CONTEXT (commander, colors, card list)
//     alongside the analysis so external tools don't need a
//     second API call to interpret the strategy fields.
//  2. Schema-versioned — external consumers can pin against
//     FreyaExportSchemaVersion and detect breaking changes via
//     the major-version bump.
//  3. Surfaces R60 extras (#416 system_tags, #421 user
//     feedback, #429 version hash) so consumers see the full
//     classification context, not just what Freya thought.
//
// Returns 404 when strategy.json doesn't exist (unlike /analysis
// which kicks off a Freya run async — export is a READ-ONLY
// contract; external tooling expects clear failure rather than
// "come back in 30 seconds").
//
// Response codes:
//   - 200 application/json — full envelope
//   - 400 — invalid owner or id
//   - 404 — deck not found OR analysis not yet run
//   - 500 — IO or unmarshal error
//
// To kick off a Freya run when no analysis exists, callers should
// POST to /api/decks/{owner}/{id}/analyze separately. Keeping the
// READ endpoint side-effect-free makes it safe to scrape across
// the corpus at integration time.
func (h *Handler) handleFreyaExport(w http.ResponseWriter, r *http.Request) {
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

	strategyPath := filepath.Join(h.DecksDir, owner, "freya", id+".strategy.json")
	analysisBytes, err := os.ReadFile(strategyPath)
	if err != nil {
		writeError(w, http.StatusNotFound,
			"freya analysis not available — POST /api/decks/"+owner+"/"+id+"/analyze first")
		return
	}
	// Validate the strategy file is parseable JSON before wrapping;
	// otherwise we'd ship a syntactically-broken envelope to the
	// integrator.
	if !json.Valid(analysisBytes) {
		writeError(w, http.StatusInternalServerError, "strategy.json is not valid JSON")
		return
	}

	// Deck context. Card list parsed once; commander + color come
	// from the deck file metadata.
	deckBytes, err := os.ReadFile(deckPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "read deck: "+err.Error())
		return
	}
	cards := parseDeckList(string(deckBytes))
	commanderCard := extractCommander(deckPath)

	// commander + color come from the standard parser-derived hint
	// (file-name suffix encodes b3_R, etc.) so external consumers
	// see the same identity hexapi uses internally.
	base := filepath.Base(deckPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	commander, _, color := parseDeckFilename(stem)
	if cc := strings.TrimSpace(commanderCard); cc != "" {
		commander = strings.ToUpper(cc)
	}

	// R60 extras.
	systemTags := loadFreyaSystemTags(h.DecksDir, owner, id)
	userFeedback := ""
	if h.db != nil {
		userFeedback = h.loadArchetypeFeedback(r.Context(), owner, id)
	}

	versionHash := ""
	versionNumber := 0
	if dag, derr := versioning.LoadDAG(filepath.Join(h.DecksDir, ".versions")); derr == nil {
		if head := dag.GetHead(owner, id); head != nil {
			versionHash = head.Hash
			versionNumber = head.Version
		}
	}

	envelope := FreyaExportEnvelope{
		SchemaVersion: FreyaExportSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:        "hexdek.dev/freya",
		Deck: FreyaExportDeck{
			Owner:         owner,
			ID:            id,
			Commander:     commander,
			CommanderCard: commanderCard,
			ColorIdentity: color,
			CardCount:     deckCardCount(cards),
			Cards:         cards,
		},
		Analysis: json.RawMessage(analysisBytes),
		Extras: FreyaExportExtras{
			SystemTags:            systemTags,
			UserArchetypeFeedback: userFeedback,
			VersionHash:           versionHash,
			VersionNumber:         versionNumber,
		},
	}

	writeJSON(w, envelope)
}

// deckCardCount totals the quantity-weighted card count from the
// parsed deck list. parseDeckList stores quantity as an int under
// the "quantity" key (with defaults of 1 when not specified).
func deckCardCount(cards []map[string]any) int {
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
