package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// apikey_test.go — algorithm + format invariants for the API key
// primitives. Lifecycle tests live in apikey_lifecycle_test.go;
// middleware-dispatch tests in apikey_middleware_test.go.

// --- GenerateAPIKey -------------------------------------------------------

func TestGenerateAPIKey_ReturnsValidTriple(t *testing.T) {
	pt, hash, id, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if pt == "" || hash == "" || id == "" {
		t.Errorf("missing piece: pt=%q hash=%q id=%q", pt, hash, id)
	}
}

func TestGenerateAPIKey_PlaintextHasPrefixAndLength(t *testing.T) {
	pt, _, _, _ := GenerateAPIKey()
	if !strings.HasPrefix(pt, APIKeyPrefix) {
		t.Errorf("plaintext %q missing %q prefix", pt, APIKeyPrefix)
	}
	if len(pt) != APIKeyTotalLen {
		t.Errorf("plaintext length %d, want %d", len(pt), APIKeyTotalLen)
	}
}

func TestGenerateAPIKey_BodyIsLowerHex(t *testing.T) {
	pt, _, _, _ := GenerateAPIKey()
	body := strings.TrimPrefix(pt, APIKeyPrefix)
	if _, err := hex.DecodeString(body); err != nil {
		t.Errorf("body %q is not valid hex: %v", body, err)
	}
	for _, c := range body {
		if c >= 'A' && c <= 'Z' {
			t.Errorf("body %q contains uppercase %q — should be lower hex", body, c)
		}
	}
}

func TestGenerateAPIKey_HashMatchesPlaintextSHA256(t *testing.T) {
	pt, hash, _, _ := GenerateAPIKey()
	sum := sha256.Sum256([]byte(pt))
	want := hex.EncodeToString(sum[:])
	if hash != want {
		t.Errorf("hash mismatch: got %q, want %q", hash, want)
	}
}

func TestGenerateAPIKey_IDLength(t *testing.T) {
	_, _, id, _ := GenerateAPIKey()
	if len(id) != APIKeyIDHexLen {
		t.Errorf("id length = %d, want %d", len(id), APIKeyIDHexLen)
	}
	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("id %q is not valid hex: %v", id, err)
	}
}

func TestGenerateAPIKey_DistinctPerCall(t *testing.T) {
	pt1, hash1, id1, _ := GenerateAPIKey()
	pt2, hash2, id2, _ := GenerateAPIKey()
	if pt1 == pt2 {
		t.Errorf("two GenerateAPIKey calls returned the same plaintext")
	}
	if hash1 == hash2 {
		t.Errorf("two GenerateAPIKey calls returned the same hash")
	}
	if id1 == id2 {
		t.Errorf("two GenerateAPIKey calls returned the same id")
	}
}

// --- HashAPIKey -----------------------------------------------------------

func TestHashAPIKey_IsDeterministic(t *testing.T) {
	const sample = "hxk_0123456789abcdef0123456789abcdef0123456789abcdef"
	a := HashAPIKey(sample)
	b := HashAPIKey(sample)
	if a != b {
		t.Errorf("hash differs across calls: %q vs %q", a, b)
	}
}

func TestHashAPIKey_DiffersForDifferentInputs(t *testing.T) {
	a := HashAPIKey("hxk_aaa")
	b := HashAPIKey("hxk_bbb")
	if a == b {
		t.Errorf("different inputs hashed to same value: %q", a)
	}
}

func TestHashAPIKey_FixedLength(t *testing.T) {
	// SHA-256 → 32 bytes → 64 hex chars regardless of input length.
	for _, in := range []string{"", "x", strings.Repeat("z", 10000)} {
		h := HashAPIKey(in)
		if len(h) != 64 {
			t.Errorf("hash of %q-length %d input is %d chars, want 64",
				in, len(in), len(h))
		}
	}
}

// --- IsAPIKeyShape --------------------------------------------------------

func TestIsAPIKeyShape_AcceptsFreshlyMintedKey(t *testing.T) {
	pt, _, _, _ := GenerateAPIKey()
	if !IsAPIKeyShape(pt) {
		t.Errorf("freshly minted key %q failed shape check", pt)
	}
}

func TestIsAPIKeyShape_RejectsSessionTokenShape(t *testing.T) {
	// Session tokens are 96 hex chars with no prefix — IsAPIKeyShape
	// must return false so the dispatcher routes them to ValidateSession.
	sessionShape := strings.Repeat("a", 96)
	if IsAPIKeyShape(sessionShape) {
		t.Errorf("session-shape token %q matched IsAPIKeyShape", sessionShape)
	}
}

func TestIsAPIKeyShape_RejectsWrongLength(t *testing.T) {
	for _, bad := range []string{
		"",
		"hxk_",
		"hxk_short",
		"hxk_" + strings.Repeat("a", APIKeyHexLen-1),  // 1 short
		"hxk_" + strings.Repeat("a", APIKeyHexLen+1),  // 1 long
		strings.Repeat("a", APIKeyTotalLen),           // right len, no prefix
	} {
		if IsAPIKeyShape(bad) {
			t.Errorf("bad shape %q matched IsAPIKeyShape", bad)
		}
	}
}

func TestIsAPIKeyShape_RejectsWrongPrefix(t *testing.T) {
	// Right length but wrong prefix — GitHub's ghp_ etc.
	body := strings.Repeat("a", APIKeyHexLen)
	for _, prefix := range []string{"ghp_", "sk__", "abc_", "    "} {
		if IsAPIKeyShape(prefix + body) {
			t.Errorf("prefix %q matched IsAPIKeyShape", prefix)
		}
	}
}

// --- Constant-time validation pattern -------------------------------------

func TestHashAPIKey_EnablesConstantTimeCompare(t *testing.T) {
	// Sanity check on the security model: validation must compare
	// HASHES, not plaintexts. A plaintext lookup would necessarily
	// be a string compare with potentially-variable timing; the
	// hash compare is fixed-width.
	pt, hash, _, _ := GenerateAPIKey()
	rehash := HashAPIKey(pt)
	if rehash != hash {
		t.Errorf("re-hashing plaintext produced %q, want %q", rehash, hash)
	}
	// Wrong plaintext → different hash. Compare via byte-equality
	// (not string-prefix matching).
	wrong := HashAPIKey("hxk_" + strings.Repeat("0", APIKeyHexLen))
	if wrong == hash {
		t.Errorf("wrong plaintext produced same hash %q (rand broken?)", hash)
	}
}
