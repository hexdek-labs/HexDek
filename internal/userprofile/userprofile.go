// Package userprofile stores per-owner profile fields that aren't part
// of the deck or session model. Today the only field is country_code,
// detected from the Accept-Language header on first identified visit.
//
// The schema is keyed by "owner" — the same slug the leaderboard and
// deck-archive screens use (data/decks/{owner}/{deck}.txt → owner).
package userprofile

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EnsureSchema creates the user_profile table if it doesn't exist.
// Safe to call repeatedly.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS user_profile (
    owner        TEXT PRIMARY KEY,
    country_code TEXT,
    detected_at  INTEGER
);`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

// SetCountryIfMissing inserts the country code for owner only if no
// row exists yet — first-detection wins, subsequent visits don't
// overwrite (e.g. user travelling, VPN). Use SetCountry to force.
func SetCountryIfMissing(ctx context.Context, db *sql.DB, owner, code string) error {
	if owner == "" || code == "" {
		return nil
	}
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO user_profile (owner, country_code, detected_at) VALUES (?, ?, ?)`,
		owner, code, time.Now().Unix())
	return err
}

// SetCountry forces an upsert. Used by the explicit profile endpoint
// where the user is claiming an owner identity.
func SetCountry(ctx context.Context, db *sql.DB, owner, code string) error {
	if owner == "" || code == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_profile (owner, country_code, detected_at)
		VALUES (?, ?, ?)
		ON CONFLICT(owner) DO UPDATE SET
		  country_code = excluded.country_code,
		  detected_at  = excluded.detected_at`,
		owner, code, time.Now().Unix())
	return err
}

// GetCountry returns the stored country code for owner, or "" if none.
func GetCountry(ctx context.Context, db *sql.DB, owner string) (string, error) {
	if owner == "" {
		return "", nil
	}
	var code string
	err := db.QueryRowContext(ctx,
		`SELECT country_code FROM user_profile WHERE owner = ?`, owner,
	).Scan(&code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return code, nil
}

// BulkCountries returns a map of owner→country_code for the supplied
// owners. Owners without a stored country are absent from the result.
// Empty input returns an empty map (no DB hit).
func BulkCountries(ctx context.Context, db *sql.DB, owners []string) (map[string]string, error) {
	out := make(map[string]string, len(owners))
	if len(owners) == 0 {
		return out, nil
	}
	// Deduplicate to keep the IN list tight.
	seen := make(map[string]bool, len(owners))
	args := make([]any, 0, len(owners))
	placeholders := make([]string, 0, len(owners))
	for _, o := range owners {
		if o == "" || seen[o] {
			continue
		}
		seen[o] = true
		args = append(args, o)
		placeholders = append(placeholders, "?")
	}
	if len(args) == 0 {
		return out, nil
	}
	q := `SELECT owner, country_code FROM user_profile WHERE owner IN (` +
		strings.Join(placeholders, ",") + `)`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner, code string
		if err := rows.Scan(&owner, &code); err != nil {
			return nil, err
		}
		if code != "" {
			out[owner] = code
		}
	}
	return out, rows.Err()
}

// ParseAcceptLanguage extracts a 2-letter ISO 3166-1 alpha-2 country
// code from an Accept-Language header. Returns "" if none can be
// determined.
//
// Strategy: pick the highest-quality tag with an explicit region
// subtag (en-US, pt-BR). If no tag has a region, infer from the
// language-only tag via languageDefaultCountry (e.g. "pl" → "PL",
// "ja" → "JP"). Hyphens and underscores are both accepted.
func ParseAcceptLanguage(header string) string {
	if header == "" {
		return ""
	}
	type pref struct {
		tag string
		q   float64
	}
	var prefs []pref
	for _, raw := range strings.Split(header, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		q := 1.0
		if i := strings.Index(raw, ";"); i >= 0 {
			tag := strings.TrimSpace(raw[:i])
			rest := strings.TrimSpace(raw[i+1:])
			if strings.HasPrefix(rest, "q=") {
				if parsed, err := parseQ(rest[2:]); err == nil {
					q = parsed
				}
			}
			raw = tag
		}
		if raw == "*" || raw == "" {
			continue
		}
		prefs = append(prefs, pref{tag: raw, q: q})
	}
	// Stable sort by q desc preserves header order on ties.
	for i := 1; i < len(prefs); i++ {
		for j := i; j > 0 && prefs[j-1].q < prefs[j].q; j-- {
			prefs[j-1], prefs[j] = prefs[j], prefs[j-1]
		}
	}
	// Pass 1: prefer tags with an explicit region.
	for _, p := range prefs {
		if c := regionFromTag(p.tag); c != "" {
			return c
		}
	}
	// Pass 2: infer from language-only.
	for _, p := range prefs {
		lang := strings.ToLower(strings.SplitN(normalizeTag(p.tag), "-", 2)[0])
		if c := languageDefaultCountry[lang]; c != "" {
			return c
		}
	}
	return ""
}

func parseQ(s string) (float64, error) {
	// Accept-Language q-values are 0..1 with up to 3 decimals; do a
	// hand-roll to avoid pulling strconv into a hot path needlessly.
	var out float64
	div := 1.0
	dot := false
	for _, r := range s {
		switch {
		case r == '.':
			if dot {
				return 0, fmt.Errorf("bad q")
			}
			dot = true
		case r >= '0' && r <= '9':
			d := float64(r - '0')
			if dot {
				div *= 10
				out += d / div
			} else {
				out = out*10 + d
			}
		default:
			return 0, fmt.Errorf("bad q")
		}
	}
	return out, nil
}

func normalizeTag(tag string) string {
	return strings.ReplaceAll(tag, "_", "-")
}

func regionFromTag(tag string) string {
	parts := strings.Split(normalizeTag(tag), "-")
	for _, p := range parts[1:] {
		if len(p) == 2 {
			isAlpha := true
			for _, r := range p {
				if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
					isAlpha = false
					break
				}
			}
			if isAlpha {
				return strings.ToUpper(p)
			}
		}
	}
	return ""
}

// languageDefaultCountry maps an ISO 639-1 language code to its most
// common country. Best-effort fallback when the user sends a bare
// language tag (e.g. "pl" → "PL", "fr" → "FR"). Add entries as needed.
var languageDefaultCountry = map[string]string{
	"en": "US", "es": "ES", "fr": "FR", "de": "DE", "it": "IT",
	"pt": "PT", "nl": "NL", "sv": "SE", "no": "NO", "da": "DK",
	"fi": "FI", "is": "IS", "pl": "PL", "cs": "CZ", "sk": "SK",
	"hu": "HU", "ro": "RO", "bg": "BG", "el": "GR", "uk": "UA",
	"ru": "RU", "be": "BY", "tr": "TR", "ar": "SA", "he": "IL",
	"fa": "IR", "ur": "PK", "hi": "IN", "bn": "BD", "ta": "IN",
	"th": "TH", "vi": "VN", "id": "ID", "ms": "MY", "tl": "PH",
	"ja": "JP", "ko": "KR", "zh": "CN", "ca": "ES", "eu": "ES",
	"gl": "ES", "ga": "IE", "cy": "GB", "et": "EE", "lv": "LV",
	"lt": "LT", "sl": "SI", "hr": "HR", "sr": "RS", "mk": "MK",
	"sq": "AL", "sw": "KE", "af": "ZA", "zu": "ZA",
}

// ─────────────────────────── Middleware ────────────────────────────

type ctxKey struct{ name string }

var countryCtxKey = ctxKey{name: "userprofile/country"}

// CountryFromContext returns the country code parsed from the
// request's Accept-Language header, or "" if no middleware ran.
func CountryFromContext(ctx context.Context) string {
	c, _ := ctx.Value(countryCtxKey).(string)
	return c
}

// LocaleMiddleware parses Accept-Language on every request and stashes
// the detected country code on the context. Handlers can retrieve it
// with CountryFromContext or via the convenience SetForOwner helpers.
func LocaleMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := ParseAcceptLanguage(r.Header.Get("Accept-Language"))
		if code != "" {
			r = r.WithContext(context.WithValue(r.Context(), countryCtxKey, code))
		}
		next.ServeHTTP(w, r)
	})
}

// ─────────────────────────── HTTP handler ──────────────────────────

// Register wires the user-profile endpoint onto mux.
//
//	POST /api/user/profile/country  body: {"owner": "..."}
//	  Persists the request-detected country code for owner.
//	  First-write wins (subsequent calls are no-ops) so a VPN user
//	  doesn't repeatedly flip flags.
func Register(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/user/profile/country", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Owner string `json:"owner"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Owner) == "" {
			http.Error(w, "missing owner", http.StatusBadRequest)
			return
		}
		owner := strings.TrimSpace(body.Owner)
		code := CountryFromContext(r.Context())
		if code == "" {
			// Fallback: parse the header here in case middleware wasn't installed.
			code = ParseAcceptLanguage(r.Header.Get("Accept-Language"))
		}
		if code == "" {
			writeJSON(w, map[string]any{"owner": owner, "country_code": "", "detected": false})
			return
		}
		if err := SetCountryIfMissing(r.Context(), db, owner, code); err != nil {
			http.Error(w, "persist: "+err.Error(), http.StatusInternalServerError)
			return
		}
		stored, _ := GetCountry(r.Context(), db, owner)
		writeJSON(w, map[string]any{"owner": owner, "country_code": stored, "detected": true})
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
