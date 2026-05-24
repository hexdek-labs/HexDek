// Package auth provides session-token issuance and validation for hexdek.
//
// Tokens are opaque random hex strings stored in the session table. They
// authenticate API and WebSocket connections to a specific device. Tokens
// can have an expiry (or 0 for non-expiring) and are revoked by deletion.
//
// Why not JWT: signing-key management adds operational burden the MVP
// doesn't need. SQLite lookup per request is fine at this scale; we can
// migrate to signed tokens later if perf becomes an issue.
package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hexdek/hexdek/internal/db"
)

// Session represents an issued authentication token.
type Session struct {
	Token      string
	DeviceID   string
	CreatedAt  int64
	ExpiresAt  int64 // 0 = no expiry
	LastUsedAt int64
}

// IssueSession creates a new session token for the given device.
// expirySeconds=0 means the token never expires.
func IssueSession(ctx context.Context, database *sql.DB, deviceID string, expirySeconds int64) (*Session, error) {
	now := db.Now()
	expires := int64(0)
	if expirySeconds > 0 {
		expires = now + expirySeconds
	}
	s := &Session{
		Token:      db.NewID(48), // 96-char hex string = 48 bytes of entropy
		DeviceID:   deviceID,
		CreatedAt:  now,
		ExpiresAt:  expires,
		LastUsedAt: now,
	}
	_, err := database.ExecContext(ctx,
		`INSERT INTO session (token, device_id, created_at, expires_at, last_used_at) VALUES (?, ?, ?, ?, ?)`,
		s.Token, s.DeviceID, s.CreatedAt, s.ExpiresAt, s.LastUsedAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	return s, nil
}

// ValidateSession looks up a token and returns its session if valid.
// Updates last_used_at as a side effect.
func ValidateSession(ctx context.Context, database *sql.DB, token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	s := &Session{}
	err := database.QueryRowContext(ctx,
		`SELECT token, device_id, created_at, expires_at, last_used_at FROM session WHERE token = ?`, token,
	).Scan(&s.Token, &s.DeviceID, &s.CreatedAt, &s.ExpiresAt, &s.LastUsedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("validate session: %w", err)
	}
	now := db.Now()
	if s.ExpiresAt > 0 && now > s.ExpiresAt {
		return nil, ErrSessionExpired
	}
	// touch last_used_at (best-effort, not transactional)
	_, _ = database.ExecContext(ctx, `UPDATE session SET last_used_at = ? WHERE token = ?`, now, token)
	s.LastUsedAt = now
	return s, nil
}

// RevokeSession deletes a session token.
func RevokeSession(ctx context.Context, database *sql.DB, token string) error {
	_, err := database.ExecContext(ctx, `DELETE FROM session WHERE token = ?`, token)
	return err
}

// RevokeAllForDevice deletes every session token issued to the given
// device. This is the "log out everywhere on this device" primitive —
// when a user clicks logout in one browser tab they expect all sibling
// tabs (and any phantom tokens left over from background refreshes) to
// stop authenticating. Returns the number of tokens revoked so the
// caller can surface it in audit logs ("revoked 4 sessions for
// device_id=X"). An empty deviceID is rejected to avoid an accidental
// mass-revoke that would log out the whole fleet.
func RevokeAllForDevice(ctx context.Context, database *sql.DB, deviceID string) (int64, error) {
	if deviceID == "" {
		return 0, errors.New("auth: empty deviceID")
	}
	res, err := database.ExecContext(ctx,
		`DELETE FROM session WHERE device_id = ?`, deviceID)
	if err != nil {
		return 0, fmt.Errorf("revoke all for device: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		// Driver doesn't support RowsAffected — the delete still ran,
		// surface that with count=0 rather than failing the caller.
		return 0, nil
	}
	return n, nil
}

// ExpireSessions deletes every session row whose expires_at is in the
// past (and non-zero — expires_at=0 means "never expires" by convention,
// see IssueSession). Returns the number of rows reaped so periodic
// callers can log a metric. Safe to call concurrently with
// ValidateSession; the worst case is a session that was about to be
// reaped instead getting a final "expired" error from a racing
// validation, which is the same answer the client would have received
// either way. Uses the idx_session_expires index for efficiency.
func ExpireSessions(ctx context.Context, database *sql.DB) (int64, error) {
	now := db.Now()
	res, err := database.ExecContext(ctx,
		`DELETE FROM session WHERE expires_at > 0 AND expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("expire sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// ErrInvalidToken is returned for missing, malformed, or unknown tokens.
var ErrInvalidToken = errors.New("invalid or unknown session token")

// ErrSessionExpired is returned for tokens past their expires_at.
var ErrSessionExpired = errors.New("session token expired")
