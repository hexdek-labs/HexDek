package hexapi

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"os"
)

// handleArchetypeFeedback accepts user feedback on Freya's
// system-assigned archetype tag (PR #416). Routed at POST
// /api/decks/{owner}/{id}/archetype-feedback under CSRF.
//
// Body:
//
//	{"action": "confirm"}                    — user agrees with Freya
//	{"action": "correct", "archetype": "X"}  — user disagrees; X is the override
//	{"action": "clear"}                      — user retracts any prior feedback
//
// On success, persists the new state to deck_meta.archetype_feedback
// AND appends a row to archetype_feedback_log (same TX). The log is
// the durable training signal — future Freya recalibration passes
// can sample it to spot recurring "Freya says X, humans always
// correct to Y" patterns.
//
// Auth: requires X-HexDek-Owner matching the path owner. Confirming
// or correcting someone else's deck's archetype is meaningless
// (only the deck owner has the context to judge), so cross-owner
// requests get 403.
//
// Response codes:
//   - 200 + {"feedback": "..."} on success
//   - 400 — malformed body, bad action, or correct without archetype
//   - 401 — missing X-HexDek-Owner
//   - 403 — caller is not the deck owner
//   - 404 — deck doesn't exist
//   - 500 — DB error
//   - 503 — DB unavailable
func (h *Handler) handleArchetypeFeedback(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	owner := r.PathValue("owner")
	id := r.PathValue("id")
	if !validatePathComponent(owner) || !validatePathComponent(id) {
		writeError(w, http.StatusBadRequest, "invalid owner or id")
		return
	}

	caller := strings.TrimSpace(strings.ToLower(r.Header.Get("X-HexDek-Owner")))
	if caller == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if !strings.EqualFold(caller, owner) {
		writeError(w, http.StatusForbidden, "only the deck owner can confirm or correct its archetype")
		return
	}

	if findDeckFile(h.DecksDir, owner, id) == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	var body struct {
		Action    string `json:"action"`
		Archetype string `json:"archetype"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "malformed body")
		return
	}
	action := strings.TrimSpace(strings.ToLower(body.Action))
	if action != "confirm" && action != "correct" && action != "clear" {
		writeError(w, http.StatusBadRequest, `action must be "confirm", "correct", or "clear"`)
		return
	}

	var override string
	if action == "correct" {
		override = normalizeSystemTagValue(body.Archetype)
		if override == "" {
			writeError(w, http.StatusBadRequest, `correct action requires a non-empty "archetype"`)
			return
		}
	}

	// Snapshot the current Freya archetype for the log row. Reads
	// strategy.json directly rather than going through
	// loadFreyaSystemTags so we capture the RAW value (e.g. "Lands
	// Matter") instead of the prefixed normalized tag — the training
	// log keeps fidelity for cross-referencing against archetypes.go.
	freyaArchetype := loadFreyaPrimaryArchetype(h.DecksDir, owner, id)

	if err := h.saveArchetypeFeedback(r.Context(), owner, id, freyaArchetype, action, override); err != nil {
		writeError(w, http.StatusInternalServerError, "save: "+err.Error())
		return
	}

	stored := ""
	switch action {
	case "confirm":
		stored = ArchetypeFeedbackConfirmed
	case "correct":
		stored = override
	}
	writeJSON(w, map[string]any{
		"owner":           owner,
		"id":              id,
		"action":          action,
		"feedback":        stored,
		"freya_archetype": freyaArchetype,
	})
}

// loadFreyaPrimaryArchetype reads strategy.json and returns the raw
// primary_archetype string (e.g. "Combo / Infinite", "Lands Matter")
// without normalization. Used by the feedback log so future Freya
// re-training can cross-reference against the canonical names in
// cmd/hexdek-freya/archetypes.go. Returns "" when strategy.json is
// missing or unparseable.
func loadFreyaPrimaryArchetype(decksDir, owner, id string) string {
	if decksDir == "" || owner == "" || id == "" {
		return ""
	}
	p := filepath.Join(decksDir, owner, "freya", id+".strategy.json")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var s struct {
		PrimaryArchetype string `json:"primary_archetype"`
	}
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s.PrimaryArchetype)
}
