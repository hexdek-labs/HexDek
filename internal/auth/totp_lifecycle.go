package auth

// totp_lifecycle.go — SQLite-backed enrollment lifecycle for the
// TOTP primitives in totp.go. Three states for a device:
//
//   (none)     no device_totp row exists                      → 2FA off
//   pending    row exists with status='pending'               → enrolled
//                                                                but not
//                                                                yet active
//   active     row exists with status='active'                → 2FA on
//
// Transitions:
//
//   EnrollTOTP   (none|pending|active) → pending
//                                        (atomically replaces any
//                                        existing row — re-enrollment
//                                        invalidates the previous
//                                        secret, which is the right
//                                        behavior if a user lost
//                                        access and is restarting)
//   ConfirmTOTP  pending(secret matches submitted code) → active
//   DisableTOTP  any → (none)
//
// The verify-code-against-stored-secret operation deliberately lives
// at the lifecycle layer rather than as a separate handler so the
// SQLite read + algorithm run share one function — eliminates the
// time-of-check/time-of-use window where a row could be DisableTOTP'd
// between fetch and verify.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// TOTPStatus represents the 2FA enrollment state of a single device.
// A nil pointer return from GetTOTPStatus means "no row exists" —
// equivalent to "2FA is off." Active and Pending are distinguished
// so the frontend can show "finish setup" vs "set up 2FA" prompts.
type TOTPStatus struct {
	DeviceID    string
	Status      string // "pending" | "active"
	CreatedAt   int64
	ConfirmedAt int64 // 0 when status=pending
}

const (
	totpStatusPending = "pending"
	totpStatusActive  = "active"
)

// EnrollTOTP starts (or restarts) 2FA enrollment for the given
// device. Returns the freshly minted secret in base32 form so the
// caller can render it as an otpauth:// URI / QR code.
//
// REPLACES any existing row — re-enrollment is by design destructive
// (user lost their authenticator and is starting over). Callers
// concerned about accidental re-enrollment by an attacker who's
// already inside the session should add a "are you sure?" prompt at
// the UX layer; the storage layer's correctness is "the secret you
// see is the secret that's active."
func EnrollTOTP(ctx context.Context, database *sql.DB, deviceID string) (secret string, err error) {
	if deviceID == "" {
		return "", errors.New("auth: empty deviceID")
	}
	secret, err = GenerateSecret()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	// Upsert so re-enroll is idempotent for the row identity but
	// destructive for the secret value. status reset to pending +
	// confirmed_at zeroed so a partial-prior-confirm doesn't leak.
	_, err = database.ExecContext(ctx, `
		INSERT INTO device_totp (device_id, secret, status, created_at, confirmed_at)
		VALUES (?, ?, ?, ?, 0)
		ON CONFLICT(device_id) DO UPDATE SET
		  secret       = excluded.secret,
		  status       = excluded.status,
		  created_at   = excluded.created_at,
		  confirmed_at = 0
	`, deviceID, secret, totpStatusPending, now)
	if err != nil {
		return "", fmt.Errorf("auth: enroll totp: %w", err)
	}
	return secret, nil
}

// ConfirmTOTP validates `code` against the device's pending TOTP
// secret. On success, transitions the row to status='active' so
// subsequent VerifyTOTP calls can rely on the secret being intended-
// for-use. Returns ErrTOTPInvalid for any code mismatch (including
// when no enrollment exists; callers MUST NOT distinguish "no
// enrollment" from "wrong code" in user-visible output).
//
// Double-confirm is rejected with ErrTOTPAlreadyActive so a stale
// frontend retry can't silently re-confirm against a different code
// the user typed in a second tab.
func ConfirmTOTP(ctx context.Context, database *sql.DB, deviceID, code string) error {
	if deviceID == "" {
		return errors.New("auth: empty deviceID")
	}
	var (
		secret string
		status string
	)
	err := database.QueryRowContext(ctx,
		`SELECT secret, status FROM device_totp WHERE device_id = ?`, deviceID).
		Scan(&secret, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTOTPInvalid
	}
	if err != nil {
		return fmt.Errorf("auth: confirm totp: lookup: %w", err)
	}
	if status == totpStatusActive {
		return ErrTOTPAlreadyActive
	}
	raw, err := DecodeSecret(secret)
	if err != nil {
		// Stored secret corrupted — surface as invalid so the user
		// can re-enroll. The actual cause goes to the server log.
		return ErrTOTPInvalid
	}
	if err := VerifyCode(raw, code, time.Now(), TOTPDefaultWindow); err != nil {
		return ErrTOTPInvalid
	}
	now := time.Now().Unix()
	_, err = database.ExecContext(ctx,
		`UPDATE device_totp SET status = ?, confirmed_at = ? WHERE device_id = ?`,
		totpStatusActive, now, deviceID)
	if err != nil {
		return fmt.Errorf("auth: confirm totp: activate: %w", err)
	}
	return nil
}

// VerifyTOTP checks a code against an ACTIVE enrollment. Returns
// ErrTOTPNotEnrolled when the device has no row or only a pending
// one — distinct from ErrTOTPInvalid so the caller can decide
// whether to fall back to a "2FA not configured" path or surface
// "wrong code". The two error classes carry different information
// to the SERVER; user-visible output should collapse them to one
// generic message so an attacker doesn't learn enrollment state.
//
// Reads + verifies in one function so a racing DisableTOTP can't
// leave a verified code falsely accepted between fetch + check.
func VerifyTOTP(ctx context.Context, database *sql.DB, deviceID, code string) error {
	if deviceID == "" {
		return ErrTOTPNotEnrolled
	}
	var (
		secret string
		status string
	)
	err := database.QueryRowContext(ctx,
		`SELECT secret, status FROM device_totp WHERE device_id = ?`, deviceID).
		Scan(&secret, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTOTPNotEnrolled
	}
	if err != nil {
		return fmt.Errorf("auth: verify totp: lookup: %w", err)
	}
	if status != totpStatusActive {
		return ErrTOTPNotEnrolled
	}
	raw, err := DecodeSecret(secret)
	if err != nil {
		return ErrTOTPInvalid
	}
	return VerifyCode(raw, code, time.Now(), TOTPDefaultWindow)
}

// GetTOTPStatus returns the current enrollment row, or nil if no
// enrollment exists. The nil-pointer return is the canonical
// "2FA is off" signal; callers should treat (nil, nil) as a
// successful query that found no row, distinct from (nil, err)
// which is a real lookup failure.
func GetTOTPStatus(ctx context.Context, database *sql.DB, deviceID string) (*TOTPStatus, error) {
	if deviceID == "" {
		return nil, errors.New("auth: empty deviceID")
	}
	s := &TOTPStatus{DeviceID: deviceID}
	err := database.QueryRowContext(ctx,
		`SELECT status, created_at, confirmed_at FROM device_totp WHERE device_id = ?`,
		deviceID).Scan(&s.Status, &s.CreatedAt, &s.ConfirmedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("auth: get totp status: %w", err)
	}
	return s, nil
}

// DisableTOTP removes the enrollment row, returning the device to
// the no-2FA state. Idempotent — calling against a device with no
// enrollment returns nil. Returns the number of rows removed so
// audit logs can distinguish "actually turned off" from "no-op."
func DisableTOTP(ctx context.Context, database *sql.DB, deviceID string) (int64, error) {
	if deviceID == "" {
		return 0, errors.New("auth: empty deviceID")
	}
	res, err := database.ExecContext(ctx,
		`DELETE FROM device_totp WHERE device_id = ?`, deviceID)
	if err != nil {
		return 0, fmt.Errorf("auth: disable totp: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil // driver doesn't support — succeeded but count unknown
	}
	return n, nil
}

// ErrTOTPAlreadyActive — a Confirm was attempted against an
// enrollment that's already active. Callers should treat this as
// "no action needed" rather than an error to surface to the user.
var ErrTOTPAlreadyActive = errors.New("auth: totp enrollment already active")

// ErrTOTPNotEnrolled — Verify was called against a device with no
// active enrollment. Distinct from ErrTOTPInvalid for server-side
// auditing; the user-visible response should collapse them.
var ErrTOTPNotEnrolled = errors.New("auth: totp not enrolled")
