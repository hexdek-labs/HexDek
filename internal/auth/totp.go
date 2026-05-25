package auth

// totp.go — RFC 6238 TOTP (HMAC-SHA1, 6 digits, 30-second period).
//
// Why hand-rolled vs a library: the algorithm is ~40 lines, the Go
// stdlib provides every primitive needed (crypto/hmac, crypto/sha1,
// crypto/rand, encoding/base32, encoding/binary), and a hand-rolled
// implementation keeps the cryptographic substrate in the same
// audit window as the consumers. Pulling in an external dep for this
// would buy nothing.
//
// Spec choices (matching what every standard authenticator app
// expects so users can scan a QR with Google Authenticator / Authy
// / 1Password / Bitwarden without algorithm-mismatch errors):
//
//   - Hash:    HMAC-SHA1 (RFC 4226 §5 + RFC 6238 §1.2)
//   - Period:  30 seconds
//   - Digits:  6
//   - Secret:  160 bits (RFC 4226 §4 recommends ≥128 bits;
//              160 = SHA-1 output size, which is what HMAC keys
//              naturally fit)
//   - Encoding: base32 (RFC 3548, padding-stripped — standard for
//              QR-shareable authenticator secrets)
//
// Window tolerance: VerifyCode walks ±N periods around the current
// step to absorb client/server clock drift. RFC 6238 §5.2 explicitly
// recommends a one-step window. We default to that but expose it
// so tests can pin specific drift scenarios.
//
// SEE ALSO: docs/auth-2fa-survey.md for what's in scope vs deferred.
// This file is the algorithm; totp_lifecycle.go wires it to the
// SQLite store; HTTP endpoints are explicitly NOT in this PR (see
// the survey for why).

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// TOTPSecretBytes is the binary length of a freshly generated
	// secret before base32 encoding. 160 bits matches the SHA-1
	// output size — RFC 4226 §4's recommendation.
	TOTPSecretBytes = 20

	// TOTPPeriodSeconds is the time step T. 30 seconds is the
	// universal authenticator-app default.
	TOTPPeriodSeconds = 30

	// TOTPDigits is the truncated code length. 6 is the universal
	// default; 8 would also work but no mainstream app defaults to
	// it, so we'd silently produce codes the user can't enter.
	TOTPDigits = 6

	// TOTPDefaultWindow is how many ±periods around the current
	// step VerifyCode accepts. 1 period = ±30s, total tolerance 90s.
	// Matches RFC 6238 §5.2's explicit recommendation.
	TOTPDefaultWindow = 1
)

// totpEncoding is base32 with padding stripped — the de-facto format
// authenticator apps display and accept. Decoding tolerates
// whitespace + case-insensitivity via the wrapper in DecodeSecret.
var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a fresh TOTPSecretBytes-byte secret encoded
// as base32 (no padding). The encoded form is what gets stored,
// rendered in QR codes, and shown to the user as a fallback if they
// can't scan the QR.
func GenerateSecret() (string, error) {
	raw := make([]byte, TOTPSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("totp: rand: %w", err)
	}
	return totpEncoding.EncodeToString(raw), nil
}

// DecodeSecret returns the binary form of a base32-encoded TOTP
// secret. Accepts the user-friendly variants authenticator apps emit
// (mixed case, embedded spaces, optional padding) so a copy-pasted
// secret with stray whitespace still decodes. Returns an error on
// any other malformation.
func DecodeSecret(encoded string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(encoded, " ", ""))
	cleaned = strings.TrimRight(cleaned, "=")
	if cleaned == "" {
		return nil, errors.New("totp: empty secret")
	}
	raw, err := totpEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("totp: decode secret: %w", err)
	}
	return raw, nil
}

// GenerateCode returns the 6-digit TOTP code for `secret` at the
// given Unix time. Lower-level entry point used by VerifyCode's
// window walk; callers wanting "the current code" should use it
// with time.Now().Unix().
//
// Implements RFC 4226 §5.3 dynamic truncation: take the low nibble
// of the last byte as offset, read 4 bytes starting there, mask the
// MSB (per spec, to avoid signed-int interpretation differences),
// modulo 10^TOTPDigits.
func GenerateCode(secret []byte, unixSeconds int64) string {
	step := unixSeconds / TOTPPeriodSeconds
	var stepBytes [8]byte
	binary.BigEndian.PutUint64(stepBytes[:], uint64(step))

	mac := hmac.New(sha1.New, secret)
	mac.Write(stepBytes[:])
	digest := mac.Sum(nil)

	offset := digest[len(digest)-1] & 0x0F
	binCode := (uint32(digest[offset]&0x7F) << 24) |
		(uint32(digest[offset+1]) << 16) |
		(uint32(digest[offset+2]) << 8) |
		uint32(digest[offset+3])

	modulus := uint32(1)
	for i := 0; i < TOTPDigits; i++ {
		modulus *= 10
	}
	code := binCode % modulus
	// Left-pad with zeros so the wire format always has TOTPDigits
	// characters — clients that just compare strings won't trip
	// over "00123" vs "123".
	return fmt.Sprintf("%0*d", TOTPDigits, code)
}

// VerifyCode returns nil if `code` matches a TOTP for `secret`
// within ±window periods of the given timestamp. window=0 means
// "this exact period only"; window>0 absorbs the named amount of
// clock drift in each direction.
//
// Comparison is constant-time (subtle-like via hmac.Equal on the
// encoded code bytes) so a timing-channel attacker can't probe
// digit-by-digit. ErrTOTPInvalid is returned for both
// length-mismatch and value-mismatch — caller MUST NOT distinguish
// these in any user-visible error or it leaks "you got the length
// right" to a brute-forcer.
func VerifyCode(secret []byte, code string, at time.Time, window int) error {
	if len(secret) == 0 {
		return ErrTOTPInvalid
	}
	if len(code) != TOTPDigits {
		// Constant-time-ish: still do the work to avoid early-exit
		// timing differences leaking "your code was the wrong
		// length" from the underlying compare path.
		_ = GenerateCode(secret, at.Unix())
		return ErrTOTPInvalid
	}
	if window < 0 {
		window = 0
	}
	base := at.Unix()
	for i := -window; i <= window; i++ {
		candidate := GenerateCode(secret, base+int64(i)*TOTPPeriodSeconds)
		if hmac.Equal([]byte(candidate), []byte(code)) {
			return nil
		}
	}
	return ErrTOTPInvalid
}

// OtpauthURI returns the otpauth:// URI a QR generator turns into
// a scannable code for `issuer` (the brand name) and `account`
// (typically the user's display name or email). The URI carries
// the secret + algorithm + digits + period parameters so a future
// algorithm migration doesn't strand authenticators that scanned
// the old version.
//
// Format: otpauth://totp/<label>?secret=<>&issuer=<>&algorithm=SHA1&digits=6&period=30
// where <label> is "<issuer>:<account>" with each component
// percent-encoded.
func OtpauthURI(issuer, account, secret string) string {
	if issuer == "" {
		issuer = "HexDek"
	}
	if account == "" {
		account = "user"
	}
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", TOTPDigits))
	q.Set("period", fmt.Sprintf("%d", TOTPPeriodSeconds))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// ErrTOTPInvalid is the single error TOTP-verification surfaces —
// length, format, and value mismatches all collapse to this so the
// wire response can't be used as a brute-force oracle.
var ErrTOTPInvalid = errors.New("totp: invalid code")
