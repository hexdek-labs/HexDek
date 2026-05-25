package hexapi

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// csrf_test.go — CSRF token primitives + middleware + integration on the
// destructive endpoints.

func newTestCSRFStore(t *testing.T) *CSRFStore {
	t.Helper()
	secret, err := RandomCSRFSecret()
	if err != nil {
		t.Fatal(err)
	}
	return NewCSRFStore(secret, time.Hour)
}

// --- Constructor invariants -----------------------------------------------

func TestNewCSRFStore_PanicsOnEmptySecret(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty secret")
		}
	}()
	_ = NewCSRFStore(nil, time.Hour)
}

func TestNewCSRFStore_DefaultTTLApplies(t *testing.T) {
	s := NewCSRFStore([]byte("secret"), 0)
	if s.ttl != time.Hour {
		t.Errorf("ttl = %v, want 1h default", s.ttl)
	}
}

func TestRandomCSRFSecret_Length(t *testing.T) {
	b, err := RandomCSRFSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 32 {
		t.Errorf("secret len = %d, want 32", len(b))
	}
}

// --- Issue + Verify happy path -------------------------------------------

func TestCSRFStore_IssueThenVerifyRoundtrip(t *testing.T) {
	s := newTestCSRFStore(t)
	tok, exp, err := s.IssueToken()
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if tok == "" {
		t.Fatal("issued token is empty")
	}
	if exp.Before(time.Now()) {
		t.Errorf("expiry %v should be in the future", exp)
	}
	if err := s.VerifyToken(tok); err != nil {
		t.Errorf("freshly-issued token failed verify: %v", err)
	}
}

func TestCSRFStore_DistinctTokensPerIssuance(t *testing.T) {
	s := newTestCSRFStore(t)
	t1, _, _ := s.IssueToken()
	t2, _, _ := s.IssueToken()
	if t1 == t2 {
		t.Fatal("two issuances returned the same token — nonce isn't fresh")
	}
}

// --- Verify failure modes -------------------------------------------------

func TestCSRFStore_VerifyEmptyTokenRejected(t *testing.T) {
	s := newTestCSRFStore(t)
	if err := s.VerifyToken(""); err != ErrCSRFMissing {
		t.Errorf("empty: want ErrCSRFMissing, got %v", err)
	}
}

func TestCSRFStore_VerifyMalformedTokenRejected(t *testing.T) {
	s := newTestCSRFStore(t)
	for _, bad := range []string{
		"not-base64-at-all!!!",                          // invalid base64 chars
		base64.RawURLEncoding.EncodeToString([]byte{1}), // wrong length
		base64.RawURLEncoding.EncodeToString(make([]byte, csrfTokenLen+1)),
	} {
		if err := s.VerifyToken(bad); err != ErrCSRFMalformed {
			t.Errorf("malformed %q: want ErrCSRFMalformed, got %v", bad, err)
		}
	}
}

func TestCSRFStore_VerifyTamperedHMACRejected(t *testing.T) {
	s := newTestCSRFStore(t)
	tok, _, _ := s.IssueToken()
	raw, _ := base64.RawURLEncoding.DecodeString(tok)
	// Flip a bit in the HMAC tail.
	raw[len(raw)-1] ^= 0x01
	bad := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.VerifyToken(bad); err != ErrCSRFBadSignature {
		t.Errorf("tampered HMAC: want ErrCSRFBadSignature, got %v", err)
	}
}

func TestCSRFStore_VerifyTamperedNonceRejected(t *testing.T) {
	s := newTestCSRFStore(t)
	tok, _, _ := s.IssueToken()
	raw, _ := base64.RawURLEncoding.DecodeString(tok)
	raw[0] ^= 0xFF
	bad := base64.RawURLEncoding.EncodeToString(raw)
	// The HMAC was computed over the original nonce, so flipping a
	// nonce byte must also surface as a signature mismatch (NOT a
	// silent acceptance of the attacker's substitution).
	if err := s.VerifyToken(bad); err != ErrCSRFBadSignature {
		t.Errorf("tampered nonce: want ErrCSRFBadSignature, got %v", err)
	}
}

func TestCSRFStore_VerifyExpiredTokenRejected(t *testing.T) {
	// Inject a clock so we can advance past expiry without sleeping.
	s := NewCSRFStore([]byte("s"), 10*time.Minute)
	base := time.Now()
	s.now = func() time.Time { return base }
	tok, _, _ := s.IssueToken()
	if err := s.VerifyToken(tok); err != nil {
		t.Fatalf("token should verify at issuance time, got %v", err)
	}
	s.now = func() time.Time { return base.Add(11 * time.Minute) }
	if err := s.VerifyToken(tok); err != ErrCSRFExpired {
		t.Errorf("past expiry: want ErrCSRFExpired, got %v", err)
	}
}

func TestCSRFStore_ExpiryCheckAfterHMAC(t *testing.T) {
	// Bug-class regression: if expiry were checked BEFORE HMAC, an
	// attacker tampering with the expiry field could leak "the rest
	// of the token looks fine" via ErrCSRFExpired vs ErrCSRFBadSignature.
	// Verify both classes surface as ErrCSRFBadSignature when the
	// HMAC doesn't match, regardless of whether the embedded expiry
	// is past or future.
	s := newTestCSRFStore(t)
	tok, _, _ := s.IssueToken()
	raw, _ := base64.RawURLEncoding.DecodeString(tok)
	// Overwrite expiry with a long-past timestamp.
	binary.BigEndian.PutUint64(raw[csrfNonceLen:csrfNonceLen+csrfExpiryLen], uint64(1))
	bad := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.VerifyToken(bad); err != ErrCSRFBadSignature {
		t.Errorf("tampered expiry must fail at HMAC check first, got %v", err)
	}
}

func TestCSRFStore_VerifyWithDifferentSecretRejected(t *testing.T) {
	s1 := NewCSRFStore([]byte("alice-secret-32-bytes-padding!!!"), time.Hour)
	s2 := NewCSRFStore([]byte("mallory-secret-32-bytes-padding!"), time.Hour)
	tok, _, _ := s1.IssueToken()
	if err := s2.VerifyToken(tok); err != ErrCSRFBadSignature {
		t.Errorf("foreign-secret token: want ErrCSRFBadSignature, got %v", err)
	}
}

// --- RequireCSRF middleware ----------------------------------------------

func TestRequireCSRF_NilStoreIsPassthrough(t *testing.T) {
	called := false
	wrapped := RequireCSRF(nil, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("DELETE", "/x", nil) // no token
	rr := httptest.NewRecorder()
	wrapped(rr, req)
	if !called {
		t.Fatal("nil store should pass through to next handler")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestRequireCSRF_MissingHeaderRejected(t *testing.T) {
	s := newTestCSRFStore(t)
	called := false
	wrapped := RequireCSRF(s, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("DELETE", "/x", nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)
	if called {
		t.Fatal("missing token: next handler MUST NOT be called")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "csrf") {
		t.Errorf("body should mention csrf, got %s", rr.Body.String())
	}
}

func TestRequireCSRF_ValidTokenPassed(t *testing.T) {
	s := newTestCSRFStore(t)
	tok, _, _ := s.IssueToken()
	called := false
	wrapped := RequireCSRF(s, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("DELETE", "/x", nil)
	req.Header.Set(CSRFHeader, tok)
	rr := httptest.NewRecorder()
	wrapped(rr, req)
	if !called {
		t.Fatal("valid token: next handler should be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}

func TestRequireCSRF_QueryParamTokenRejected(t *testing.T) {
	// Security pin: the verifier MUST NOT accept tokens via query
	// param or form body. An attacker who can forge the URL can forge
	// the param, defeating CSRF protection. Only X-CSRF-Token header.
	s := newTestCSRFStore(t)
	tok, _, _ := s.IssueToken()
	wrapped := RequireCSRF(s, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("DELETE", "/x?csrf_token="+tok, nil)
	// Deliberately do NOT set the header.
	rr := httptest.NewRecorder()
	wrapped(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("query-param token must NOT authenticate, got %d", rr.Code)
	}
}

// --- HandleIssueCSRF ------------------------------------------------------

func TestHandleIssueCSRF_NilStoreReturns503(t *testing.T) {
	h := HandleIssueCSRF(nil)
	req := httptest.NewRequest("GET", "/api/csrf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 (clear opt-out signal)", rr.Code)
	}
}

func TestHandleIssueCSRF_ReturnsUsableToken(t *testing.T) {
	s := newTestCSRFStore(t)
	h := HandleIssueCSRF(s)
	req := httptest.NewRequest("GET", "/api/csrf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if cc := rr.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store (token must not be cached)", cc)
	}
	var resp struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
		Header    string `json:"header"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("response token is empty")
	}
	if resp.Header != CSRFHeader {
		t.Errorf("response.header = %q, want %q", resp.Header, CSRFHeader)
	}
	if resp.ExpiresAt < time.Now().Unix() {
		t.Errorf("expires_at %d should be in the future", resp.ExpiresAt)
	}
	// Round-trip: the issued token verifies against the same store.
	if err := s.VerifyToken(resp.Token); err != nil {
		t.Errorf("issued token failed verify: %v", err)
	}
}

// --- Integration: DELETE deck via Register() with the wrapper ------------

func newCSRFIntegrationHandler(t *testing.T, withStore bool) (*Handler, string, *CSRFStore) {
	t.Helper()
	tmp, err := os.MkdirTemp("", "hexapi-csrf-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	decksDir := filepath.Join(tmp, "decks", "alice")
	if err := os.MkdirAll(decksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	deckPath := filepath.Join(decksDir, "korvold.txt")
	if err := os.WriteFile(deckPath, []byte("COMMANDER: Korvold\n1 Sol Ring\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &Handler{DecksDir: filepath.Join(tmp, "decks")}
	var store *CSRFStore
	if withStore {
		store = newTestCSRFStore(t)
		h.CSRFStore = store
	}
	return h, deckPath, store
}

func TestIntegration_DeleteDeck_NilStorePassesWithoutToken(t *testing.T) {
	// Backwards-compat pin: when CSRFStore is nil, the existing flow
	// (X-HexDek-Owner ownership check only) keeps working.
	h, deckPath, _ := newCSRFIntegrationHandler(t, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("DELETE", "/api/decks/alice/korvold", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s — nil store must pass through", rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(deckPath); !os.IsNotExist(err) {
		t.Errorf("deck file should have been deleted, stat err = %v", err)
	}
}

func TestIntegration_DeleteDeck_StoreSetRequiresToken(t *testing.T) {
	// With a store wired, a request WITHOUT X-CSRF-Token must 403 even
	// when the ownership header is correct — the CSRF gate sits in
	// front of the ownership check.
	h, deckPath, _ := newCSRFIntegrationHandler(t, true)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("DELETE", "/api/decks/alice/korvold", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — store set without token MUST reject", rr.Code)
	}
	if _, err := os.Stat(deckPath); err != nil {
		t.Errorf("deck file must still exist after rejected DELETE, got err = %v", err)
	}
}

func TestIntegration_DeleteDeck_ValidTokenAccepted(t *testing.T) {
	h, deckPath, store := newCSRFIntegrationHandler(t, true)
	mux := http.NewServeMux()
	h.Register(mux)

	tok, _, _ := store.IssueToken()
	req := httptest.NewRequest("DELETE", "/api/decks/alice/korvold", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	req.Header.Set(CSRFHeader, tok)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s — valid token + owner should succeed",
			rr.Code, rr.Body.String())
	}
	if _, err := os.Stat(deckPath); !os.IsNotExist(err) {
		t.Errorf("deck file should have been deleted, stat err = %v", err)
	}
}

func TestIntegration_DeleteDeck_TamperedTokenRejected(t *testing.T) {
	h, deckPath, store := newCSRFIntegrationHandler(t, true)
	mux := http.NewServeMux()
	h.Register(mux)

	tok, _, _ := store.IssueToken()
	// Flip last char of base64 — corrupts the HMAC tail.
	bad := tok[:len(tok)-1] + "A"
	if bad == tok {
		bad = tok[:len(tok)-1] + "B"
	}
	req := httptest.NewRequest("DELETE", "/api/decks/alice/korvold", nil)
	req.Header.Set("X-HexDek-Owner", "alice")
	req.Header.Set(CSRFHeader, bad)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("tampered token: status = %d, want 403", rr.Code)
	}
	if _, err := os.Stat(deckPath); err != nil {
		t.Errorf("deck file must still exist, err = %v", err)
	}
}

// --- /api/csrf issuance round-trip via Register() ------------------------

func TestIntegration_IssueCSRFEndpoint_Registered(t *testing.T) {
	h, _, _ := newCSRFIntegrationHandler(t, true)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/csrf", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var resp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Token == "" {
		t.Fatal("empty token in response")
	}
}

func TestIntegration_IssueCSRFEndpoint_503WhenStoreNil(t *testing.T) {
	h, _, _ := newCSRFIntegrationHandler(t, false)
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest("GET", "/api/csrf", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when CSRFStore is nil", rr.Code)
	}
}

// Sanity: response body shape is stable across changes — guards
// frontend code that parses it.
func TestHandleIssueCSRF_ResponseShape(t *testing.T) {
	s := newTestCSRFStore(t)
	h := HandleIssueCSRF(s)
	req := httptest.NewRequest("GET", "/api/csrf", nil)
	rr := httptest.NewRecorder()
	h(rr, req)
	var dec map[string]any
	if err := json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&dec); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"token", "expires_at", "header"} {
		if _, ok := dec[key]; !ok {
			t.Errorf("response missing key %q (full=%+v)", key, dec)
		}
	}
}
