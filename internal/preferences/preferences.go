// Package preferences stores per-device user-facing UI preferences
// surfaced through GET / PATCH /api/me/preferences. Distinct from
// internal/userprofile (which is keyed on the owner slug and stores
// country-code metadata derived from Accept-Language); preferences
// here are keyed on the authenticated device_id and store fields the
// user types into the Profile screen — display name + owner name.
//
// Closes docs/half-finished-features-r48.md #8. Profile.jsx used to
// persist these only via localStorage, with a "stored locally in your
// browser only" caveat; this package wires real server-side sync
// through the auth identity so preferences survive across devices.
package preferences

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hexdek/hexdek/internal/auth"
)

// Preferences is the per-device profile-screen state. Keyed on the
// authenticated device_id from the auth Session. UpdatedAt is the
// unix-seconds upsert timestamp.
type Preferences struct {
	DeviceID    string `json:"device_id"`
	DisplayName string `json:"display_name"`
	OwnerName   string `json:"owner_name"`
	UpdatedAt   int64  `json:"updated_at"`
}

// EnsureSchema creates the user_preferences table if missing. Safe to
// call repeatedly. The display_name + owner_name columns are
// nullable-friendly (default empty string) so a brand-new device can
// GET and receive zero values rather than 404.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS user_preferences (
    device_id    TEXT PRIMARY KEY,
    display_name TEXT NOT NULL DEFAULT '',
    owner_name   TEXT NOT NULL DEFAULT '',
    updated_at   INTEGER NOT NULL DEFAULT 0
);`
	_, err := db.ExecContext(ctx, ddl)
	return err
}

// Get returns the stored preferences for deviceID. A non-existent row
// returns (Preferences{DeviceID: deviceID}, nil) — i.e. the zero-value
// shape rather than sql.ErrNoRows. This makes the GET handler simpler
// (it always renders a row, never 404).
func Get(ctx context.Context, db *sql.DB, deviceID string) (Preferences, error) {
	p := Preferences{DeviceID: deviceID}
	if deviceID == "" {
		return p, nil
	}
	err := db.QueryRowContext(ctx,
		`SELECT display_name, owner_name, updated_at FROM user_preferences WHERE device_id = ?`,
		deviceID,
	).Scan(&p.DisplayName, &p.OwnerName, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return Preferences{}, err
	}
	return p, nil
}

// PatchInput is the body of PATCH /api/me/preferences. Nullable so a
// caller can update just one field without clobbering the other; both
// fields nil = no-op (the handler returns the current row unchanged).
type PatchInput struct {
	DisplayName *string `json:"display_name,omitempty"`
	OwnerName   *string `json:"owner_name,omitempty"`
}

// Patch applies the non-nil PatchInput fields to deviceID's row,
// inserting a fresh row when none exists. Returns the post-update
// Preferences so the caller can echo the new state back to the client.
//
// Strings are trimmed of leading/trailing whitespace. Empty strings
// ARE allowed (the user wants to clear a field); the nullable pointer
// distinguishes "clear it" (non-nil empty) from "leave it alone" (nil).
func Patch(ctx context.Context, db *sql.DB, deviceID string, in PatchInput) (Preferences, error) {
	if deviceID == "" {
		return Preferences{}, errors.New("preferences: empty device_id")
	}
	current, err := Get(ctx, db, deviceID)
	if err != nil {
		return Preferences{}, err
	}
	if in.DisplayName != nil {
		current.DisplayName = strings.TrimSpace(*in.DisplayName)
	}
	if in.OwnerName != nil {
		current.OwnerName = strings.TrimSpace(*in.OwnerName)
	}
	// Skip the write when neither field was supplied — the GET branch
	// already returned the unchanged row.
	if in.DisplayName == nil && in.OwnerName == nil {
		return current, nil
	}
	current.UpdatedAt = time.Now().Unix()
	_, err = db.ExecContext(ctx, `
		INSERT INTO user_preferences (device_id, display_name, owner_name, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(device_id) DO UPDATE SET
		  display_name = excluded.display_name,
		  owner_name   = excluded.owner_name,
		  updated_at   = excluded.updated_at`,
		deviceID, current.DisplayName, current.OwnerName, current.UpdatedAt)
	if err != nil {
		return Preferences{}, err
	}
	return current, nil
}

// Register wires the GET + PATCH endpoints onto mux. Both require an
// authenticated session (any credential: bearer token or hxk_ API key)
// — auth.Required wraps each handler and 401s unauthenticated calls
// per the audit doc's "ride on top of the auth model" requirement.
//
// Endpoints:
//
//	GET   /api/me/preferences        → returns {device_id, display_name, owner_name, updated_at}
//	PATCH /api/me/preferences        → body {display_name?, owner_name?} — null/omit fields are not modified
//
// The audit doc explicitly named /api/me/preferences as the target
// endpoint; the path is preserved verbatim so historical references
// in stub-hunt-remaining-internal-r47.md remain accurate.
func Register(mux *http.ServeMux, database *sql.DB) {
	mux.Handle("GET /api/me/preferences",
		auth.Required(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := auth.FromContext(r.Context())
			if s == nil {
				// Should be unreachable — auth.Required only forwards
				// to this handler after successful credential
				// validation — but the nil-guard avoids a panic if a
				// future refactor moves the context-stash.
				http.Error(w, "no session", http.StatusInternalServerError)
				return
			}
			p, err := Get(r.Context(), database, s.DeviceID)
			if err != nil {
				http.Error(w, "preferences get: "+err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, p)
		})))

	mux.Handle("PATCH /api/me/preferences",
		auth.Required(database, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := auth.FromContext(r.Context())
			if s == nil {
				http.Error(w, "no session", http.StatusInternalServerError)
				return
			}
			var in PatchInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
				return
			}
			p, err := Patch(r.Context(), database, s.DeviceID, in)
			if err != nil {
				http.Error(w, "preferences patch: "+err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, p)
		})))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
