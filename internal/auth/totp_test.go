package auth

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// totp_test.go — RFC 6238 algorithm coverage. The lifecycle tests
// in totp_lifecycle_test.go exercise the SQLite layer; this file
// pins the pure-algorithm contract so a future "drop in a library"
// substitution can be verified against the same vectors.

// --- RFC 6238 Appendix B test vectors ------------------------------------
//
// The RFC publishes test codes for the secret "12345678901234567890"
// (20 ASCII bytes — a "stub" not produced by GenerateSecret, but
// chosen because it directly matches the spec's worked example) at
// fixed timestamps. These vectors are the single load-bearing
// "the algorithm is correct" check: any code-generation change
// must reproduce them exactly.

func TestGenerateCode_RFC6238_AppendixB_SHA1Vectors(t *testing.T) {
	secret := []byte("12345678901234567890") // RFC §B.1

	// RFC 6238 publishes 8-digit codes; the spec also defines
	// truncation to other digit counts via the same modulus rule.
	// We're a 6-digit implementation, so we mod the published
	// 8-digit codes down to 6 by taking the last six characters
	// (equivalent to (8digit % 10^6) since the truncation is
	// least-significant).
	cases := []struct {
		unix       int64
		want8digit string // from RFC 6238 §B.1 SHA-1 column
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, tc := range cases {
		got := GenerateCode(secret, tc.unix)
		want := tc.want8digit[len(tc.want8digit)-TOTPDigits:]
		if got != want {
			t.Errorf("t=%d: got %s, want %s (truncated from RFC 8-digit %s)",
				tc.unix, got, want, tc.want8digit)
		}
	}
}

// --- GenerateCode invariants ---------------------------------------------

func TestGenerateCode_AlwaysSixDigitsZeroPadded(t *testing.T) {
	// Even when the dynamic-truncation result is a small number,
	// the wire format MUST be zero-padded to TOTPDigits so string
	// comparison works on the client side.
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := DecodeSecret(secret)
	// Walk a few hundred steps — somewhere in there a small
	// modulus result will trigger the zero-pad path.
	for unix := int64(0); unix < 30*200; unix += 30 {
		code := GenerateCode(raw, unix)
		if len(code) != TOTPDigits {
			t.Fatalf("unix=%d: code %q has length %d, want %d", unix, code, len(code), TOTPDigits)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("unix=%d: code %q has non-digit %q", unix, code, c)
			}
		}
	}
}

func TestGenerateCode_StableWithinPeriod(t *testing.T) {
	// Two timestamps inside the same 30s window MUST return the
	// same code — that's the spec's whole "T = floor(unixtime/T)"
	// floor operation. Use a period-aligned base so the +29 offset
	// stays within the window.
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	base := int64(1700000010) // 1700000010 / 30 = 56666667 exactly
	c1 := GenerateCode(raw, base)
	c2 := GenerateCode(raw, base+29) // same step
	c3 := GenerateCode(raw, base+30) // next step
	if c1 != c2 {
		t.Errorf("codes within one period differ: %s vs %s", c1, c2)
	}
	if c1 == c3 {
		t.Errorf("codes across period boundary should differ: %s == %s", c1, c3)
	}
}

// --- VerifyCode window tolerance -----------------------------------------

func TestVerifyCode_AcceptsCurrentPeriod(t *testing.T) {
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000015, 0) // mid-period
	code := GenerateCode(raw, now.Unix())
	if err := VerifyCode(raw, code, now, 0); err != nil {
		t.Errorf("current-period code rejected: %v", err)
	}
}

func TestVerifyCode_AcceptsPreviousPeriodWithinWindow(t *testing.T) {
	// User's clock is ~30s slow: they enter a code for the
	// previous period. With window=1 the verifier must accept.
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000045, 0) // in period [1700000030, 1700000060)
	prevCode := GenerateCode(raw, now.Unix()-30)
	if err := VerifyCode(raw, prevCode, now, 1); err != nil {
		t.Errorf("previous-period code with window=1 rejected: %v", err)
	}
}

func TestVerifyCode_AcceptsNextPeriodWithinWindow(t *testing.T) {
	// User's clock is ~30s fast: they enter a code for the next
	// period. window=1 must accept.
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000045, 0)
	nextCode := GenerateCode(raw, now.Unix()+30)
	if err := VerifyCode(raw, nextCode, now, 1); err != nil {
		t.Errorf("next-period code with window=1 rejected: %v", err)
	}
}

func TestVerifyCode_RejectsPeriodOutsideWindow(t *testing.T) {
	// 2 periods (60s) ago is outside the default ±1 window.
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000045, 0)
	stale := GenerateCode(raw, now.Unix()-60)
	if err := VerifyCode(raw, stale, now, 1); err != ErrTOTPInvalid {
		t.Errorf("2-period-old code with window=1 should reject; got %v", err)
	}
}

func TestVerifyCode_RejectsWrongLength(t *testing.T) {
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if err := VerifyCode(raw, bad, time.Now(), 1); err != ErrTOTPInvalid {
			t.Errorf("len-mismatch %q: want ErrTOTPInvalid, got %v", bad, err)
		}
	}
}

func TestVerifyCode_RejectsEmptySecret(t *testing.T) {
	if err := VerifyCode(nil, "123456", time.Now(), 1); err != ErrTOTPInvalid {
		t.Errorf("nil secret: want ErrTOTPInvalid, got %v", err)
	}
	if err := VerifyCode([]byte{}, "123456", time.Now(), 1); err != ErrTOTPInvalid {
		t.Errorf("empty secret: want ErrTOTPInvalid, got %v", err)
	}
}

func TestVerifyCode_RejectsWrongCode(t *testing.T) {
	// Sanity: a correctly-formed but actually-wrong code must reject.
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000015, 0)
	right := GenerateCode(raw, now.Unix())
	wrong := "000000"
	if right == wrong {
		// 1-in-a-million chance the random secret happens to
		// produce 000000 at this timestamp; bump and re-run.
		wrong = "999999"
	}
	if err := VerifyCode(raw, wrong, now, 1); err != ErrTOTPInvalid {
		t.Errorf("wrong code: want ErrTOTPInvalid, got %v", err)
	}
}

func TestVerifyCode_NegativeWindowClampedToZero(t *testing.T) {
	// Defensive: a caller passing window=-1 must NOT silently accept
	// ALL codes (or panic). Spec says clamp to "this period only."
	secret, _ := GenerateSecret()
	raw, _ := DecodeSecret(secret)
	now := time.Unix(1700000015, 0)
	right := GenerateCode(raw, now.Unix())
	if err := VerifyCode(raw, right, now, -1); err != nil {
		t.Errorf("window=-1 should still accept current-period code: %v", err)
	}
	prev := GenerateCode(raw, now.Unix()-30)
	if err := VerifyCode(raw, prev, now, -1); err != ErrTOTPInvalid {
		t.Errorf("window=-1 should reject previous-period code; got %v", err)
	}
}

// --- GenerateSecret / DecodeSecret roundtrip -----------------------------

func TestGenerateSecret_DefaultLength(t *testing.T) {
	s, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := DecodeSecret(s)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) != TOTPSecretBytes {
		t.Errorf("decoded len = %d, want %d", len(raw), TOTPSecretBytes)
	}
}

func TestGenerateSecret_IsBase32StandardNoPadding(t *testing.T) {
	s, _ := GenerateSecret()
	// No padding chars + uppercase + length consistent with 20 bytes.
	if strings.Contains(s, "=") {
		t.Errorf("secret %q contains base32 padding; should be stripped", s)
	}
	if s != strings.ToUpper(s) {
		t.Errorf("secret %q not all-uppercase", s)
	}
	// 20 bytes → 32 base32 chars without padding.
	if len(s) != 32 {
		t.Errorf("encoded len = %d, want 32 (160 bits → 32 base32 chars)", len(s))
	}
}

func TestGenerateSecret_DistinctPerCall(t *testing.T) {
	s1, _ := GenerateSecret()
	s2, _ := GenerateSecret()
	if s1 == s2 {
		t.Errorf("two GenerateSecret calls returned the same value (rand broken?)")
	}
}

func TestDecodeSecret_AcceptsUserFriendlyForm(t *testing.T) {
	// Authenticator apps display secrets in groups of 4 with spaces:
	// "JBSW Y3DP EHPK 3PXP" — must decode just like the no-space form.
	s, _ := GenerateSecret()
	// Insert spaces every 4 chars.
	var spaced strings.Builder
	for i, c := range s {
		if i > 0 && i%4 == 0 {
			spaced.WriteByte(' ')
		}
		spaced.WriteRune(c)
	}
	rawDirect, err := DecodeSecret(s)
	if err != nil {
		t.Fatal(err)
	}
	rawSpaced, err := DecodeSecret(spaced.String())
	if err != nil {
		t.Fatalf("spaced decode: %v", err)
	}
	if string(rawDirect) != string(rawSpaced) {
		t.Error("spaced + unspaced decoded to different bytes")
	}
}

func TestDecodeSecret_AcceptsLowercase(t *testing.T) {
	s, _ := GenerateSecret()
	rawUpper, _ := DecodeSecret(s)
	rawLower, err := DecodeSecret(strings.ToLower(s))
	if err != nil {
		t.Fatalf("lowercase decode: %v", err)
	}
	if string(rawUpper) != string(rawLower) {
		t.Error("uppercase + lowercase decoded to different bytes")
	}
}

func TestDecodeSecret_RejectsEmpty(t *testing.T) {
	if _, err := DecodeSecret(""); err == nil {
		t.Error("empty secret should error")
	}
	if _, err := DecodeSecret("   "); err == nil {
		t.Error("whitespace-only secret should error")
	}
}

func TestDecodeSecret_RejectsInvalidBase32(t *testing.T) {
	if _, err := DecodeSecret("!!!not-base32!!!"); err == nil {
		t.Error("garbage should fail to decode")
	}
}

// --- OtpauthURI shape ----------------------------------------------------

func TestOtpauthURI_ContainsExpectedParameters(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP" // known test secret from the wikipedia article
	uri := OtpauthURI("HexDek", "alice", secret)
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("URI prefix wrong: %s", uri)
	}
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	q := parsed.Query()
	if q.Get("secret") != secret {
		t.Errorf("secret param = %q, want %q", q.Get("secret"), secret)
	}
	if q.Get("issuer") != "HexDek" {
		t.Errorf("issuer param = %q, want HexDek", q.Get("issuer"))
	}
	if q.Get("algorithm") != "SHA1" {
		t.Errorf("algorithm = %q, want SHA1", q.Get("algorithm"))
	}
	if q.Get("digits") != "6" {
		t.Errorf("digits = %q, want 6", q.Get("digits"))
	}
	if q.Get("period") != "30" {
		t.Errorf("period = %q, want 30", q.Get("period"))
	}
}

func TestOtpauthURI_LabelIsIssuerColonAccount(t *testing.T) {
	uri := OtpauthURI("HexDek", "alice", "JBSWY3DPEHPK3PXP")
	parsed, _ := url.Parse(uri)
	// Path comes back as "/HexDek:alice" — strip the leading slash.
	label := strings.TrimPrefix(parsed.Path, "/")
	if label != "HexDek:alice" {
		t.Errorf("label = %q, want HexDek:alice", label)
	}
}

func TestOtpauthURI_DefaultsApplyOnBlankInput(t *testing.T) {
	// Blank issuer → "HexDek"; blank account → "user". Documented in
	// the function comment so callers can rely on it.
	uri := OtpauthURI("", "", "JBSWY3DPEHPK3PXP")
	parsed, _ := url.Parse(uri)
	if strings.TrimPrefix(parsed.Path, "/") != "HexDek:user" {
		t.Errorf("default label wrong: %s", parsed.Path)
	}
	if parsed.Query().Get("issuer") != "HexDek" {
		t.Error("default issuer should be HexDek")
	}
}

// --- Cross-API round-trip: enroll → code → verify ------------------------

func TestTOTP_EndToEndRoundtrip(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := DecodeSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	code := GenerateCode(raw, now.Unix())
	if err := VerifyCode(raw, code, now, TOTPDefaultWindow); err != nil {
		t.Errorf("self-generated code didn't verify: %v", err)
	}
}
