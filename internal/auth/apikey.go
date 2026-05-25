package auth

// apikey.go — API key format + hashing primitives.
//
// Why API keys alongside sessions: sessions are short-lived (30 days)
// device-scoped bearer tokens minted at device registration. They
// work for an interactive client (the SPA) but not for the use cases
// the user explicitly asked for:
//
//   - Long-lived programmatic access (CI / scripts / external
//     integrations that don't have a browser session to re-issue)
//   - Identifiable, individually-revocable credentials (a session
//     token is opaque — there's no "list my active credentials and
//     revoke just the one on my old laptop")
//   - Distinguishable in logs + git scrubbers (the "hxk_" prefix
//     makes a leaked HexDek key visually obvious — same idea as
//     GitHub's "ghp_" / "ghs_" / Stripe's "sk_live_" prefixes)
//
// Wire format:
//
//   hxk_<48 hex chars>             total 52 chars
//
// 24 random bytes (192 bits) — well above the 128-bit minimum for a
// brute-force-resistant bearer token. The full string is the secret;
// the prefix isn't a separator, it's part of the comparison.
//
// Storage: the database stores ONLY the SHA-256 hash of the full
// string. The plaintext is shown exactly once at issuance and never
// recoverable from the row. SHA-256 is the right choice here (not
// bcrypt/argon2) because the input is high-entropy random — slow
// hashing protects against guessing low-entropy passwords, which
// doesn't apply when each bit was minted by crypto/rand.
//
// Lookup is by hash equality: hash the incoming token, look it up
// in the `api_key` table by `key_hash`. Single indexed query, no
// per-row brute force. The hash column is unique-constrained.
//
// SEE ALSO: docs/auth-2fa-survey.md for the broader auth picture.
// This is the second auth credential type after sessions, and
// (unlike TOTP) it's a primary credential, not a second factor.

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	// APIKeyPrefix prefixes every issued key. Lets log scrubbers,
	// reverse-engineering attackers, and humans visually identify
	// HexDek API keys vs other bearer tokens.
	APIKeyPrefix = "hxk_"

	// APIKeyEntropyBytes is the random body length BEFORE hex
	// encoding. 24 bytes = 192 bits, well above the 128-bit floor
	// for a bearer token.
	APIKeyEntropyBytes = 24

	// APIKeyHexLen is the encoded body length (2 hex chars / byte).
	APIKeyHexLen = APIKeyEntropyBytes * 2

	// APIKeyTotalLen is len(prefix) + APIKeyHexLen — the exact
	// length every plaintext key should be. Used in IsAPIKeyShape
	// for a length check before doing any heavier work.
	APIKeyTotalLen = len(APIKeyPrefix) + APIKeyHexLen

	// APIKeyIDBytes is the row-lookup id length BEFORE hex encoding.
	// The id is a separate handle from the key itself — it's what
	// shows up in "revoke key XYZ" UX so users don't have to paste
	// their secret to identify it. 6 bytes / 12 hex chars is plenty
	// to disambiguate within a single user's small set of keys.
	APIKeyIDBytes = 6

	// APIKeyIDHexLen — encoded id length, what gets stored in
	// api_key.id and used as the URL path component for revoke.
	APIKeyIDHexLen = APIKeyIDBytes * 2
)

// GenerateAPIKey returns a freshly minted key triple:
//
//   - plaintext: the full "hxk_..." string. SHOW TO THE USER ONCE,
//     NEVER STORE. After issuance the plaintext is unrecoverable
//     (intentionally — leaking the storage layer must not leak
//     usable credentials).
//   - hash: SHA-256 hex of the plaintext. This is what goes into
//     api_key.key_hash for the equality lookup.
//   - id: separate short hex handle for revoke UX (api_key.id +
//     the {id} path component on DELETE /api/auth/keys/{id}). The
//     id is NOT a secret — it can appear in logs and admin views
//     without exposing the key.
//
// Both random reads come from crypto/rand. A failure on either is
// a hard fail — we'd rather fail closed than issue a low-entropy
// "fallback" key.
func GenerateAPIKey() (plaintext, hash, id string, err error) {
	body := make([]byte, APIKeyEntropyBytes)
	if _, err := rand.Read(body); err != nil {
		return "", "", "", fmt.Errorf("apikey: rand: %w", err)
	}
	idBytes := make([]byte, APIKeyIDBytes)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", "", fmt.Errorf("apikey: rand id: %w", err)
	}
	plaintext = APIKeyPrefix + hex.EncodeToString(body)
	hash = HashAPIKey(plaintext)
	id = hex.EncodeToString(idBytes)
	return plaintext, hash, id, nil
}

// HashAPIKey returns the SHA-256 hex of the plaintext key. Used by
// both the issuer (to store) and the validator (to look up). The
// hash itself is fixed-length (64 hex chars) regardless of input,
// so the storage column can be a plain TEXT without size dynamics.
//
// SHA-256 is the right primitive here because:
//
//   - The input is a 192-bit cryptographically random value, so
//     dictionary attacks are mathematically infeasible. Slow
//     hashes (bcrypt/argon2) defend against low-entropy passwords;
//     they don't add value for high-entropy randoms.
//   - We need single-shot, server-side hashing on every validation
//     — making it expensive would add latency to every API call.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// IsAPIKeyShape returns true if the input looks like a plausible
// API key by prefix + length. Used by the middleware to dispatch
// between session-token validation and API-key validation without
// doing two failed DB lookups for every request.
//
// This is a CHEAP gate — it doesn't verify the hex chars are valid
// or that the key exists. A plausible-shaped string that doesn't
// validate will hit ErrInvalidAPIKey at the validation layer
// regardless.
func IsAPIKeyShape(s string) bool {
	if len(s) != APIKeyTotalLen {
		return false
	}
	return strings.HasPrefix(s, APIKeyPrefix)
}

// ErrInvalidAPIKey is the single error API-key validation surfaces.
// Length, format, and hash-mismatch all collapse to this — same
// principle as ErrTOTPInvalid: the wire response must not become a
// brute-force oracle distinguishing "wrong length" from "wrong
// value" or "key revoked" from "key never existed."
var ErrInvalidAPIKey = errors.New("apikey: invalid or unknown key")

// ErrAPIKeyExpired — key validated by hash but its expires_at is
// past. Surfaced separately from ErrInvalidAPIKey so the SERVER
// can audit "this previously-valid key just expired." Caller MUST
// collapse to a single user-visible "unauthorized" so the
// distinction doesn't leak.
var ErrAPIKeyExpired = errors.New("apikey: expired")

// ErrAPIKeyRevoked — key was issued, validated by hash, but its
// revoked_at is set. Same audit-vs-user reasoning as ErrAPIKeyExpired.
var ErrAPIKeyRevoked = errors.New("apikey: revoked")
