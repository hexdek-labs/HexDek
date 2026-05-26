package hexapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"time"
)

// CSRFStore mints and verifies stateless CSRF tokens for mutating
// endpoints. Tokens are signed with an HMAC over (nonce || expiry),
// so verification is O(1) and needs no server-side bookkeeping — a
// restart with a fresh secret simply invalidates outstanding tokens
// (acceptable for the 1-hour default expiry).
//
// Threat model: this is belt-and-suspenders on top of the existing
// custom-header pattern (X-HexDek-Owner). Because every mutating
// hexapi endpoint requires either a JSON body or an X-HexDek-Owner
// header, the browser already issues a CORS preflight that blocks
// cross-origin form CSRF. CSRF tokens add defense against:
//   - A future CORS misconfiguration that loosens the origin allow-
//     list (e.g. echoing Origin into Access-Control-Allow-Origin).
//   - XSS-derived requests that share the victim's origin but don't
//     know the token (the token lives in sessionStorage / memory,
//     not in a cookie that a script can lift trivially).
//
// Token shape: base64url( nonce[16] || expiry_be8 || hmac[32] ) —
// 56 bytes raw, ~75 chars encoded. Opaque to clients; they echo it
// in the X-CSRF-Token request header on writes.
type CSRFStore struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time // injectable for tests
}

const (
	csrfNonceLen  = 16
	csrfExpiryLen = 8
	csrfHMACLen   = 32
	csrfTokenLen  = csrfNonceLen + csrfExpiryLen + csrfHMACLen

	// CSRFHeader is the HTTP header clients echo the token in.
	CSRFHeader = "X-CSRF-Token"
)

var (
	// ErrCSRFMissing — no token presented (header empty / not a string).
	ErrCSRFMissing = errors.New("csrf: missing token")
	// ErrCSRFMalformed — token is not valid base64 or is the wrong length.
	ErrCSRFMalformed = errors.New("csrf: malformed token")
	// ErrCSRFExpired — token decoded cleanly but its embedded expiry is past.
	ErrCSRFExpired = errors.New("csrf: token expired")
	// ErrCSRFBadSignature — HMAC didn't match (tampered or wrong secret).
	ErrCSRFBadSignature = errors.New("csrf: bad signature")
)

// NewCSRFStore returns a store keyed off the given secret. The secret
// MUST be unguessable; callers usually generate 32 random bytes at
// startup (RandomCSRFSecret) or load it from an env var so tokens
// survive restarts. Panics on a zero-length secret — fail fast rather
// than silently accept everything an attacker mints. ttl is the
// per-token validity window; pass 0 for the 1-hour default.
func NewCSRFStore(secret []byte, ttl time.Duration) *CSRFStore {
	if len(secret) == 0 {
		panic("hexapi.NewCSRFStore: empty secret")
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	cp := make([]byte, len(secret))
	copy(cp, secret)
	return &CSRFStore{secret: cp, ttl: ttl, now: time.Now}
}

// RandomCSRFSecret returns 32 cryptographically random bytes suitable
// for NewCSRFStore. Wired into cmd/hexdek-server when HEXDEK_CSRF_SECRET
// is unset, so dev environments get a per-boot secret automatically.
func RandomCSRFSecret() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

// IssueToken mints a fresh CSRF token. Each call returns a distinct
// token (the nonce is freshly random) — callers do not need to cache.
func (s *CSRFStore) IssueToken() (string, time.Time, error) {
	expiry := s.now().Add(s.ttl).Unix()
	raw := make([]byte, csrfTokenLen)
	if _, err := rand.Read(raw[:csrfNonceLen]); err != nil {
		return "", time.Time{}, err
	}
	binary.BigEndian.PutUint64(raw[csrfNonceLen:csrfNonceLen+csrfExpiryLen], uint64(expiry))
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(raw[:csrfNonceLen+csrfExpiryLen])
	copy(raw[csrfNonceLen+csrfExpiryLen:], mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString(raw), time.Unix(expiry, 0), nil
}

// VerifyToken returns nil if the token is well-formed, unexpired, and
// signed with this store's secret. On failure, returns one of four
// sentinel errors (ErrCSRFMissing / ErrCSRFMalformed / ErrCSRFBadSignature
// / ErrCSRFExpired). HTTP handlers should collapse all four into a
// generic 403 so the wire response can't distinguish "no token" from
// "bad signature"; the sentinel is preserved server-side for audit logs.
func (s *CSRFStore) VerifyToken(tok string) error {
	if tok == "" {
		return ErrCSRFMissing
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil || len(raw) != csrfTokenLen {
		return ErrCSRFMalformed
	}
	expiry := int64(binary.BigEndian.Uint64(raw[csrfNonceLen : csrfNonceLen+csrfExpiryLen]))
	want := hmac.New(sha256.New, s.secret)
	want.Write(raw[:csrfNonceLen+csrfExpiryLen])
	if !hmac.Equal(want.Sum(nil), raw[csrfNonceLen+csrfExpiryLen:]) {
		return ErrCSRFBadSignature
	}
	// Expiry check AFTER HMAC check — otherwise a tampered expiry would
	// leak "the rest of the token looked fine" via a different error.
	if s.now().Unix() >= expiry {
		return ErrCSRFExpired
	}
	return nil
}

// RequireCSRF returns an http.HandlerFunc that enforces the CSRF
// token on a wrapped handler. When the store is nil the wrapper is a
// straight pass-through — backwards-compatible with older binaries
// that haven't constructed a store. cmd/hexdek-server controls
// enforcement via HEXDEK_CSRF_ENFORCE so existing frontends keep
// working until they're updated to issue + send the token.
//
// The verifier reads the token from the X-CSRF-Token request header.
// We deliberately do NOT accept the token from a query parameter or
// form field — both would defeat the protection (an attacker who can
// forge a URL can forge the parameter).
func RequireCSRF(store *CSRFStore, next http.HandlerFunc) http.HandlerFunc {
	if store == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.VerifyToken(r.Header.Get(CSRFHeader)); err != nil {
			writeError(w, http.StatusForbidden, "csrf: invalid or missing token")
			return
		}
		next(w, r)
	}
}

// HandleIssueCSRF returns the issuance handler for GET /api/csrf.
// Returns 503 if the store is nil so callers know the server isn't
// participating in token-based CSRF — better than fabricating a
// useless token that won't verify.
func HandleIssueCSRF(store *CSRFStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if store == nil {
			writeError(w, http.StatusServiceUnavailable, "csrf: not configured")
			return
		}
		tok, exp, err := store.IssueToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "csrf: issue failed")
			return
		}
		// no-store: an intermediate cache must not hand the same token
		// to a different user — token is per-issuance, not per-resource.
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, map[string]any{
			"token":      tok,
			"expires_at": exp.Unix(),
			"header":     CSRFHeader,
		})
	}
}
