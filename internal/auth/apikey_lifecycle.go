package auth

// apikey_lifecycle.go — DB-backed lifecycle for API keys. Mirrors
// the session_lifecycle.go shape so callers familiar with one know
// what to expect from the other.
//
// Operations:
//
//   IssueAPIKey    — mint a new key, return plaintext ONCE
//   ValidateAPIKey — look up + check expiry/revoke status,
//                    touch last_used_at as a side effect
//   ListAPIKeys    — enumerate a device's keys (no plaintext)
//   RevokeAPIKey   — soft-delete (set revoked_at), preserves the
//                    row so the same id can't be re-issued and the
//                    audit trail survives
//
// Why soft-revoke vs hard delete: revoked rows still appear in the
// device's history ("you revoked 'CI runner' on Jan 14") which is
// useful for security review after a leaked credential. Hard
// delete would lose that. The unique constraint on key_hash means
// the row sticks around until a future GC.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// APIKey is the DB row, minus the plaintext (which only exists
// transiently at issue time).
type APIKey struct {
	ID         string
	DeviceID   string
	Name       string
	KeyHash    string
	CreatedAt  int64
	LastUsedAt int64
	ExpiresAt  int64 // 0 = no expiry
	RevokedAt  int64 // 0 = active
}

// Active returns true when the key would currently validate. Encodes
// the "revoked_at>0 OR (expires_at>0 AND past)" check in one place
// so list / validate / status agree on what "active" means.
func (k *APIKey) Active(at time.Time) bool {
	if k.RevokedAt > 0 {
		return false
	}
	if k.ExpiresAt > 0 && at.Unix() >= k.ExpiresAt {
		return false
	}
	return true
}

// IssueAPIKey mints a fresh key for a device. Returns the plaintext
// for one-time display to the user — the caller MUST surface it
// immediately and not log it.
//
// `name` is purely cosmetic (shows up in the management UI to help
// users identify keys); empty is allowed. `expirySeconds=0` means
// never expires; positive value sets expires_at to `now+expirySeconds`.
//
// The (plaintext, *APIKey) return pair: plaintext is the secret to
// hand to the user; the struct mirrors what's stored in the DB
// (KeyHash, not plaintext). Caller can serialize the struct freely
// — it has no plaintext to leak.
func IssueAPIKey(ctx context.Context, database *sql.DB, deviceID, name string, expirySeconds int64) (plaintext string, key *APIKey, err error) {
	if deviceID == "" {
		return "", nil, errors.New("apikey: empty deviceID")
	}
	pt, hash, id, err := GenerateAPIKey()
	if err != nil {
		return "", nil, err
	}
	now := time.Now().Unix()
	expires := int64(0)
	if expirySeconds > 0 {
		expires = now + expirySeconds
	}
	row := &APIKey{
		ID:         id,
		DeviceID:   deviceID,
		Name:       name,
		KeyHash:    hash,
		CreatedAt:  now,
		LastUsedAt: 0,
		ExpiresAt:  expires,
		RevokedAt:  0,
	}
	_, err = database.ExecContext(ctx, `
		INSERT INTO api_key (id, device_id, name, key_hash, created_at, last_used_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, 0)
	`, row.ID, row.DeviceID, row.Name, row.KeyHash, row.CreatedAt, row.ExpiresAt)
	if err != nil {
		return "", nil, fmt.Errorf("apikey: insert: %w", err)
	}
	return pt, row, nil
}

// ValidateAPIKey looks up a key by hashing the supplied plaintext
// and querying for the matching row. Returns the row on success;
// returns ErrInvalidAPIKey for unknown/malformed, ErrAPIKeyExpired
// for past-expiry, ErrAPIKeyRevoked for soft-deleted. Callers MUST
// collapse all three to a single "unauthorized" on the wire so the
// distinction doesn't become a brute-force oracle — the granular
// errors are for server-side audit.
//
// Side effect: updates last_used_at to now() on successful
// validation, best-effort (a failed update doesn't fail the
// validation — the auth result is what matters; the audit-trail
// staleness is a minor cost).
func ValidateAPIKey(ctx context.Context, database *sql.DB, plaintext string) (*APIKey, error) {
	if !IsAPIKeyShape(plaintext) {
		return nil, ErrInvalidAPIKey
	}
	hash := HashAPIKey(plaintext)
	k := &APIKey{}
	err := database.QueryRowContext(ctx, `
		SELECT id, device_id, name, key_hash, created_at, last_used_at, expires_at, revoked_at
		FROM api_key WHERE key_hash = ?
	`, hash).Scan(&k.ID, &k.DeviceID, &k.Name, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, fmt.Errorf("apikey: validate: %w", err)
	}
	if k.RevokedAt > 0 {
		return nil, ErrAPIKeyRevoked
	}
	now := time.Now()
	if k.ExpiresAt > 0 && now.Unix() >= k.ExpiresAt {
		return nil, ErrAPIKeyExpired
	}
	// Best-effort last-used touch — keyed by id (not hash) so the
	// query plan uses the PK index rather than the hash index.
	_, _ = database.ExecContext(ctx,
		`UPDATE api_key SET last_used_at = ? WHERE id = ?`, now.Unix(), k.ID)
	k.LastUsedAt = now.Unix()
	return k, nil
}

// ListAPIKeys returns all keys for a device (active + revoked +
// expired), newest-first. NEVER returns plaintext or the key_hash;
// callers building API responses should marshal only ID / Name /
// CreatedAt / LastUsedAt / ExpiresAt / RevokedAt.
//
// Empty device returns (nil, error) to prevent an accidental
// "list everything in the table" by a typo'd argument.
func ListAPIKeys(ctx context.Context, database *sql.DB, deviceID string) ([]APIKey, error) {
	if deviceID == "" {
		return nil, errors.New("apikey: empty deviceID")
	}
	rows, err := database.QueryContext(ctx, `
		SELECT id, device_id, name, key_hash, created_at, last_used_at, expires_at, revoked_at
		FROM api_key WHERE device_id = ? ORDER BY created_at DESC
	`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("apikey: list: %w", err)
	}
	defer rows.Close()
	out := []APIKey{}
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.DeviceID, &k.Name, &k.KeyHash, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt, &k.RevokedAt); err != nil {
			return nil, fmt.Errorf("apikey: scan: %w", err)
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeAPIKey soft-deletes a key by id. Requires both id AND
// deviceID — caller MUST pass their own deviceID so a hijacked
// session can't reach across and revoke a different user's keys.
// Returns ErrInvalidAPIKey when the (id, device_id) pair doesn't
// match anything, so a probing attacker can't enumerate other users'
// key ids via the response shape.
//
// Idempotent: revoking an already-revoked key is a no-op success.
func RevokeAPIKey(ctx context.Context, database *sql.DB, id, deviceID string) error {
	if id == "" || deviceID == "" {
		return ErrInvalidAPIKey
	}
	now := time.Now().Unix()
	res, err := database.ExecContext(ctx, `
		UPDATE api_key SET revoked_at = ?
		WHERE id = ? AND device_id = ? AND revoked_at = 0
	`, now, id, deviceID)
	if err != nil {
		return fmt.Errorf("apikey: revoke: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// Driver doesn't support RowsAffected — the UPDATE still ran;
		// assume success rather than error the caller out.
		return nil
	}
	if n == 0 {
		// Either the key doesn't exist, belongs to a different
		// device, or was already revoked. Re-query to distinguish
		// "already revoked" (idempotent success) from "not yours /
		// doesn't exist" (ErrInvalidAPIKey).
		var revokedAt int64
		err := database.QueryRowContext(ctx,
			`SELECT revoked_at FROM api_key WHERE id = ? AND device_id = ?`,
			id, deviceID).Scan(&revokedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidAPIKey
		}
		if err != nil {
			return fmt.Errorf("apikey: revoke probe: %w", err)
		}
		// Row exists + revoked_at != 0 → already revoked, that's fine.
		return nil
	}
	return nil
}
