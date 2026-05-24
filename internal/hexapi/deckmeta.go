package hexapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// deck_meta stores per-deck overrides that don't belong in the on-disk
// deck files — currently just custom_name (a user-set display title that
// overrides the derived commander/filename name).
//
// Keyed by (owner, id) which mirrors the URL shape, so the frontend can
// PATCH a deck without us having to look up an internal UUID first.

const deckMetaSchema = `
CREATE TABLE IF NOT EXISTS deck_meta (
    owner        TEXT NOT NULL,
    id           TEXT NOT NULL,
    custom_name  TEXT NOT NULL DEFAULT '',
    cloned_from  TEXT NOT NULL DEFAULT '',
    tags         TEXT NOT NULL DEFAULT '',
    updated_at   INTEGER NOT NULL,
    PRIMARY KEY (owner, id)
);

-- Per-user clone rate limiter. One row per clone attempt; the cap is
-- enforced by counting rows in the trailing hour for a given owner.
CREATE TABLE IF NOT EXISTS clone_log (
    owner       TEXT NOT NULL,
    src_key     TEXT NOT NULL,
    dst_key     TEXT NOT NULL,
    cloned_at   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_clone_log_owner_time
    ON clone_log(owner, cloned_at DESC);
`

// EnsureDeckMetaSchema creates the deck_meta table idempotently. Call once
// at startup (right after handing the *sql.DB to the Handler). Also
// idempotently adds the cloned_from column on databases that predate the
// clone feature, so existing installs don't need a migration.
func EnsureDeckMetaSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("hexapi: nil db for deck_meta schema")
	}
	if _, err := db.ExecContext(ctx, deckMetaSchema); err != nil {
		return err
	}
	// SQLite has no IF NOT EXISTS for ADD COLUMN; ignore the duplicate-
	// column error so this is safe to run repeatedly.
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE deck_meta ADD COLUMN cloned_from TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE deck_meta ADD COLUMN tags TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	return nil
}

// SetDB attaches a database to the Handler. Must be called before requests
// arrive if you want PATCH /api/decks/{owner}/{id} to persist changes.
func (h *Handler) SetDB(db *sql.DB) { h.db = db }

func (h *Handler) loadCustomName(ctx context.Context, owner, id string) string {
	if h.db == nil {
		return ""
	}
	var name string
	err := h.db.QueryRowContext(ctx,
		`SELECT custom_name FROM deck_meta WHERE owner = ? AND id = ?`,
		owner, id).Scan(&name)
	if err != nil {
		return ""
	}
	return name
}

func (h *Handler) saveCustomName(ctx context.Context, owner, id, name string) error {
	if h.db == nil {
		return errors.New("hexapi: deck_meta storage not initialized")
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO deck_meta (owner, id, custom_name, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(owner, id) DO UPDATE SET
		   custom_name = excluded.custom_name,
		   updated_at  = excluded.updated_at`,
		owner, id, name, time.Now().Unix())
	return err
}

// loadClonedFrom returns the "owner/id" pointer recorded when this deck
// was created by /clone, or "" when the deck was imported directly.
func (h *Handler) loadClonedFrom(ctx context.Context, owner, id string) string {
	if h.db == nil {
		return ""
	}
	var src string
	err := h.db.QueryRowContext(ctx,
		`SELECT cloned_from FROM deck_meta WHERE owner = ? AND id = ?`,
		owner, id).Scan(&src)
	if err != nil {
		return ""
	}
	return src
}

// saveClonedFrom records the source deck a clone was based on. Upserts
// alongside any existing custom_name so the two are stored together.
func (h *Handler) saveClonedFrom(ctx context.Context, owner, id, src string) error {
	if h.db == nil {
		return errors.New("hexapi: deck_meta storage not initialized")
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO deck_meta (owner, id, cloned_from, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(owner, id) DO UPDATE SET
		   cloned_from = excluded.cloned_from,
		   updated_at  = excluded.updated_at`,
		owner, id, src, time.Now().Unix())
	return err
}

// MaxTagsPerDeck caps how many tags a single deck may carry. Keeps the
// UI usable and prevents pathological clients from inflating deck_meta.
const MaxTagsPerDeck = 16

// MaxTagLen bounds the length of a single tag string.
const MaxTagLen = 32

// normalizeTags trims, lowercases, dedupes and length-limits an incoming
// tag list. Tag matching elsewhere is case-insensitive, so we normalize
// on the way in. Returns ("", nil) for an empty list, or the JSON-encoded
// array on success. Validation errors short-circuit with a non-nil err.
func normalizeTags(raw []string) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if len(raw) > MaxTagsPerDeck {
		return "", errors.New("too many tags (max 16)")
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" {
			continue
		}
		if len(t) > MaxTagLen {
			return "", errors.New("tag too long (max 32 chars)")
		}
		for _, c := range t {
			if c < 0x20 || c == 0x7f {
				return "", errors.New("tag contains control characters")
			}
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return "", nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// decodeTags parses the stored JSON tag array. Returns an empty slice
// for blank or malformed input so callers don't have to guard.
func decodeTags(s string) []string {
	if s == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil
	}
	return tags
}

func (h *Handler) loadTags(ctx context.Context, owner, id string) []string {
	if h.db == nil {
		return nil
	}
	var raw string
	err := h.db.QueryRowContext(ctx,
		`SELECT tags FROM deck_meta WHERE owner = ? AND id = ?`,
		owner, id).Scan(&raw)
	if err != nil {
		return nil
	}
	return decodeTags(raw)
}

func (h *Handler) saveTags(ctx context.Context, owner, id, tagsJSON string) error {
	if h.db == nil {
		return errors.New("hexapi: deck_meta storage not initialized")
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO deck_meta (owner, id, tags, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(owner, id) DO UPDATE SET
		   tags       = excluded.tags,
		   updated_at = excluded.updated_at`,
		owner, id, tagsJSON, time.Now().Unix())
	return err
}

// allTagsForOwner returns every distinct tag string used by the given
// owner, with a usage count so the autocomplete can rank common tags
// first. When owner is empty the query spans every owner — handy for
// admin tooling but the autocomplete passes the requester's owner so
// suggestions stay personal.
func (h *Handler) allTagsForOwner(ctx context.Context, owner string) ([]tagSuggestion, error) {
	if h.db == nil {
		return nil, nil
	}
	var rows *sql.Rows
	var err error
	if owner == "" {
		rows, err = h.db.QueryContext(ctx,
			`SELECT tags FROM deck_meta WHERE tags != ''`)
	} else {
		rows, err = h.db.QueryContext(ctx,
			`SELECT tags FROM deck_meta WHERE owner = ? AND tags != ''`,
			owner)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		for _, t := range decodeTags(raw) {
			counts[t]++
		}
	}
	out := make([]tagSuggestion, 0, len(counts))
	for t, n := range counts {
		out = append(out, tagSuggestion{Tag: t, Count: n})
	}
	return out, nil
}

type tagSuggestion struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// CloneRateLimit is the cap on clone-creates per user per hour. Exported
// so tests can lower it without poking at unexported internals.
const CloneRateLimit = 10

// cloneCountSince returns how many clones the given owner has created
// since the supplied unix timestamp. Used by the clone handler to
// enforce CloneRateLimit; returns 0 (i.e. allow) when the DB is absent.
func (h *Handler) cloneCountSince(ctx context.Context, owner string, since int64) (int, error) {
	if h.db == nil {
		return 0, nil
	}
	var n int
	err := h.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clone_log WHERE owner = ? AND cloned_at >= ?`,
		owner, since).Scan(&n)
	return n, err
}

// recordClone appends a row to clone_log so the rate limiter can see
// this attempt on the next call. Best-effort: a write error is logged
// elsewhere but does not roll back the clone the caller already wrote
// to disk.
func (h *Handler) recordClone(ctx context.Context, owner, srcKey, dstKey string) error {
	if h.db == nil {
		return nil
	}
	_, err := h.db.ExecContext(ctx,
		`INSERT INTO clone_log (owner, src_key, dst_key, cloned_at)
		 VALUES (?, ?, ?, ?)`,
		owner, srcKey, dstKey, time.Now().Unix())
	return err
}

// handlePatchDeck handles `PATCH /api/decks/{owner}/{id}` for lightweight
// metadata updates. Today: only `name` (the user-set display title). Other
// fields can be added later without changing the route.
//
// Body: {"name": "..."} — empty string clears the custom name and reverts
// the deck back to its derived display.
//
// Returns the updated DeckSummary-shaped JSON so the frontend can swap it
// into local state without a follow-up GET.
func (h *Handler) handlePatchDeck(w http.ResponseWriter, r *http.Request) {
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

	if findDeckFile(h.DecksDir, owner, id) == "" {
		writeError(w, http.StatusNotFound, "deck not found")
		return
	}

	var body struct {
		Name *string   `json:"name"`
		Tags *[]string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == nil && body.Tags == nil {
		writeError(w, http.StatusBadRequest, "no patchable fields supplied")
		return
	}

	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if len(name) > 120 {
			writeError(w, http.StatusBadRequest, "name too long (max 120 chars)")
			return
		}
		// Disallow control chars and embedded newlines so the name renders
		// cleanly in the UI and as a hero title.
		for _, c := range name {
			if c < 0x20 || c == 0x7f {
				writeError(w, http.StatusBadRequest, "name contains control characters")
				return
			}
		}
		if err := h.saveCustomName(r.Context(), owner, id, name); err != nil {
			writeError(w, http.StatusInternalServerError, "save: "+err.Error())
			return
		}
	}

	if body.Tags != nil {
		tagsJSON, err := normalizeTags(*body.Tags)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.saveTags(r.Context(), owner, id, tagsJSON); err != nil {
			writeError(w, http.StatusInternalServerError, "save: "+err.Error())
			return
		}
	}

	// Build a fresh summary so the client can swap state directly.
	commander, bracket, color := parseDeckFilename(id)
	customName := h.loadCustomName(r.Context(), owner, id)
	displayName := commander
	if customName != "" {
		displayName = customName
	}
	resp := map[string]any{
		"owner":       owner,
		"id":          id,
		"name":        displayName,
		"commander":   commander,
		"custom_name": customName,
		"bracket":     bracket,
		"color":       color,
		"tags":        h.loadTags(r.Context(), owner, id),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleListTags returns the distinct tags in use for autocomplete, with
// a usage count so the client can rank popular tags first.
//
// Query params:
//
//	owner=...  (optional) — restrict to one owner. Defaults to the caller's
//	                        X-HexDek-Owner header; pass owner=* to span all.
//	q=...      (optional) — case-insensitive substring filter.
//	limit=N    (optional, default 20, max 100) — cap on returned suggestions.
//
// Always returns 200 with a JSON array (possibly empty) so the client
// doesn't have to special-case "no decks yet".
func (h *Handler) handleListTags(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	owner := strings.TrimSpace(r.URL.Query().Get("owner"))
	if owner == "" {
		owner = strings.TrimSpace(strings.ToLower(r.Header.Get("X-HexDek-Owner")))
	}
	if owner == "*" {
		owner = ""
	}

	suggestions, err := h.allTagsForOwner(r.Context(), owner)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tags: "+err.Error())
		return
	}

	if q != "" {
		filtered := suggestions[:0]
		for _, s := range suggestions {
			if strings.Contains(s.Tag, q) {
				filtered = append(filtered, s)
			}
		}
		suggestions = filtered
	}
	// Most-used first, alphabetical tiebreaker so results are stable
	// across requests when counts match.
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Count != suggestions[j].Count {
			return suggestions[i].Count > suggestions[j].Count
		}
		return suggestions[i].Tag < suggestions[j].Tag
	})
	if len(suggestions) > limit {
		suggestions = suggestions[:limit]
	}
	if suggestions == nil {
		suggestions = []tagSuggestion{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(suggestions)
}
