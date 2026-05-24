package hexapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hexdek/hexdek/internal/userprofile"
)

// OwnerProfile is the public per-owner profile payload returned by
// GET /api/profile/{owner}. Distinct from the auth-user session-stats
// summary handleProfile returns at GET /api/profile (no path param).
//
// Country is a 2-letter ISO 3166-1 alpha-2 code (uppercase) — e.g. "US",
// "JP", "BR". Empty string when no preference is stored and no
// Accept-Language hint is available.
type OwnerProfile struct {
	Owner   string `json:"owner"`
	Country string `json:"country,omitempty"`
}

// profileStore is the on-disk shape we read for stored country
// preferences. Lives at <DecksDir>/../profiles/{owner}.json so it sits
// next to the existing data tree without polluting decks/. The file is
// optional — if absent, the handler falls back to Accept-Language.
type profileStore struct {
	Country string `json:"country"`
}

// profilesDir derives the on-disk path for stored profile prefs from
// the configured DecksDir. We use DecksDir's parent so a Showmatch
// running with DecksDir="data/decks" looks at "data/profiles".
func (h *Handler) profilesDir() string {
	parent := filepath.Dir(h.DecksDir)
	if parent == "" || parent == "." {
		return "profiles"
	}
	return filepath.Join(parent, "profiles")
}

// loadStoredCountry returns the stored country code for owner, or ""
// if no profile file exists / the file is unreadable. Reads the
// SQLite user_profile table first when a DB is attached, then falls
// back to the legacy JSON file at <profilesDir>/lowercased(owner).json
// so manual overrides still work. Owner is case-insensitive.
func (h *Handler) loadStoredCountry(owner string) string {
	if owner == "" {
		return ""
	}
	if h.db != nil {
		if c, err := userprofile.GetCountry(context.Background(), h.db, owner); err == nil && c != "" {
			return strings.ToUpper(c)
		}
	}
	path := filepath.Join(h.profilesDir(), strings.ToLower(owner)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var p profileStore
	if err := json.Unmarshal(data, &p); err != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(p.Country))
}

// parseAcceptLanguageCountry pulls a 2-letter country code out of an
// HTTP Accept-Language header. The header has the form
// "en-US,en;q=0.9,fr-CA;q=0.8" — we walk the comma-separated list and
// return the first tag whose region segment is exactly two letters.
//
// Returns "" when no usable country is present (e.g. "en", "*", or a
// language-only header like "fr,de").
func parseAcceptLanguageCountry(header string) string {
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if i := strings.Index(tag, ";"); i >= 0 {
			tag = tag[:i]
		}
		// language[-script]-region. Pick the LAST hyphen-separated part
		// that's exactly two ASCII letters; that's the region per
		// BCP 47 (script subtag is 4 letters, language is 2-3, so a
		// 2-letter trailing segment is unambiguously the region).
		parts := strings.Split(tag, "-")
		for j := len(parts) - 1; j >= 1; j-- {
			p := strings.TrimSpace(parts[j])
			if len(p) == 2 && isAlpha(p[0]) && isAlpha(p[1]) {
				return strings.ToUpper(p)
			}
		}
	}
	return ""
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// resolveCountry combines the stored-pref and Accept-Language paths.
// Stored prefs always win; Accept-Language is the fallback so a brand
// new owner shows a sensible flag the first time their page loads.
//
// When the country comes from Accept-Language and a SQLite store is
// attached, we opportunistically persist it (first-write-wins) so the
// next visit doesn't need to re-detect — satisfying the "store the
// detected country code in the user's SQLite profile" requirement.
func (h *Handler) resolveCountry(owner string, r *http.Request) string {
	if c := h.loadStoredCountry(owner); c != "" {
		return c
	}
	c := parseAcceptLanguageCountry(r.Header.Get("Accept-Language"))
	if c != "" && h.db != nil && owner != "" {
		_ = userprofile.SetCountryIfMissing(r.Context(), h.db, owner, c)
	}
	return c
}

// handleOwnerProfile implements GET /api/profile/{owner}.
func (h *Handler) handleOwnerProfile(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	if !validatePathComponent(owner) {
		writeError(w, http.StatusBadRequest, "invalid owner")
		return
	}
	writeJSON(w, OwnerProfile{
		Owner:   owner,
		Country: h.resolveCountry(owner, r),
	})
}

// handleOwnerProfilesBatch implements GET /api/profiles?owners=a,b,c.
// Returns a map of owner → country so the leaderboard can render a
// flag per row without N round-trips. Unknown owners are still
// included with whatever country resolveCountry returns (typically
// the requester's Accept-Language fallback) — the frontend treats an
// empty country string as "no flag".
//
// The owners list is capped at 200 entries to bound response size and
// keep one batch fetch per leaderboard load reasonable.
func (h *Handler) handleOwnerProfilesBatch(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("owners")
	if raw == "" {
		writeJSON(w, map[string]string{})
		return
	}
	parts := strings.Split(raw, ",")
	const maxBatch = 200
	if len(parts) > maxBatch {
		parts = parts[:maxBatch]
	}
	out := make(map[string]string, len(parts))
	seen := make(map[string]bool, len(parts))
	fallback := parseAcceptLanguageCountry(r.Header.Get("Accept-Language"))
	for _, p := range parts {
		owner := strings.TrimSpace(p)
		if owner == "" || seen[owner] || !validatePathComponent(owner) {
			continue
		}
		seen[owner] = true
		c := h.loadStoredCountry(owner)
		if c == "" {
			c = fallback
		}
		out[owner] = c
	}
	writeJSON(w, out)
}
