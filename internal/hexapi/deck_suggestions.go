package hexapi

import (
	"net/http"
	"os"
	"path/filepath"
)

// handleDeckSuggestions answers GET /api/decks/{owner}/{id}/suggestions
// with the deck-builder suggestion rollup (prioritized adds / cuts /
// swaps) computed by Freya's BuildSuggestedChanges. Surface for the
// Decks-screen upgrade UI: each entry carries a card name, category,
// priority, and a human-readable reason.
//
// Response shape (see cmd/hexdek-freya/suggested_changes.go for the
// canonical Go types):
//
//	{
//	  "adds": [
//	    {"card_name":"Counterspell","category":"interaction","priority":8,
//	     "reason":"current 4 interaction spells, target 6 for Control archetype","gap_size":2},
//	    ...
//	  ],
//	  "cuts": [
//	    {"card_name":"Gilded Lotus","category":"cuttable","priority":5,
//	     "reason":"high CMC with no clear role"},
//	    ...
//	  ],
//	  "swaps": [
//	    {"add_card_name":"Bountiful Promenade","cut_card_name":"Tranquil Cove",
//	     "category":"manabase","priority":7,
//	     "reason":"Mana base C-grade — swap Tranquil Cove for Bountiful Promenade ..."},
//	    ...
//	  ],
//	  "summary": "3 adds, 2 cuts, 1 swap suggested. Highest priority: ...",
//	  "total_changes": 6
//	}
//
// Disk-cached: reads the `id.suggestions.json` sidecar emitted by
// Freya's saveSuggestionsJSON alongside strategy.json / profile.json.
// Mirrors handleGetAnalysis's read-then-fallback pattern: when the
// sidecar is missing, auto-triggers a Freya analysis run if the deck
// file exists and returns a 202-style "analyzing" status payload so
// the frontend can poll without surfacing a 500 to the user.
//
// Returns 404 only when the deck FILE itself can't be found.
// Returns 400 when path components fail validation.
func (h *Handler) handleDeckSuggestions(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	freyaDir := filepath.Join(h.DecksDir, owner, "freya")
	suggestionsPath := filepath.Join(freyaDir, id+".suggestions.json")

	data, err := os.ReadFile(suggestionsPath)
	if err != nil {
		// Sidecar absent — auto-trigger Freya if the deck file exists.
		// Mirrors handleGetAnalysis's "analyzing — refresh in a few
		// seconds" pattern so the frontend can poll.
		deckPath := findDeckFile(h.DecksDir, owner, id)
		if deckPath != "" {
			go h.runFreya(deckPath)
			writeJSON(w, map[string]any{
				"status":  "analyzing",
				"message": "Freya analysis started — refresh in a few seconds",
			})
			return
		}
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
