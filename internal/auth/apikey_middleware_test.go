package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// apikey_middleware_test.go — pins the dispatch behavior added to
// Required / RequiredFunc: incoming bearer tokens go to ValidateAPIKey
// when they have the hxk_ shape, ValidateSession otherwise. Both
// produce a *Session in context so existing FromContext callers are
// unchanged.

// captureHandler is a tiny http.HandlerFunc that records the Session
// FromContext gave it. Tests use it as the "downstream" handler to
// assert what credentials the middleware passed through.
type captureHandler struct {
	called  bool
	session *Session
}

func (c *captureHandler) HandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.called = true
		c.session = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}
}

// --- Dispatch: session tokens still work ---------------------------------

func TestRequiredFunc_StillAcceptsSessionTokens(t *testing.T) {
	// Backwards-compatibility pin: the existing flow (Authorization:
	// Bearer <96-hex-char session token>) MUST still validate after
	// the API-key dispatcher landed.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	sess, err := IssueSession(context.Background(), d, alice, 3600)
	if err != nil {
		t.Fatal(err)
	}

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if !cap.called {
		t.Fatal("downstream handler not called")
	}
	if cap.session == nil || cap.session.DeviceID != alice {
		t.Errorf("FromContext returned %+v, want session for %s", cap.session, alice)
	}
}

// --- Dispatch: API keys validate -----------------------------------------

func TestRequiredFunc_AcceptsAPIKey(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, err := IssueAPIKey(context.Background(), d, alice, "ci", 0)
	if err != nil {
		t.Fatal(err)
	}

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if cap.session == nil {
		t.Fatal("FromContext returned nil — middleware didn't attach an identity")
	}
	if cap.session.DeviceID != alice {
		t.Errorf("device = %q, want %q", cap.session.DeviceID, alice)
	}
	// Synthetic Session.Token should be "apikey:<id>" so downstream
	// audit can tell credential types apart without exposing the
	// plaintext. Pin that contract.
	wantToken := "apikey:" + key.ID
	if cap.session.Token != wantToken {
		t.Errorf("synthetic token = %q, want %q", cap.session.Token, wantToken)
	}
}

func TestRequiredFunc_AcceptsAPIKeyViaQueryParam(t *testing.T) {
	// extractToken supports ?token= for WebSocket clients. That path
	// must work for API keys too (a WebSocket-driven script wouldn't
	// have an easier path than a header in any case, but the
	// extractToken contract is "either source").
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, _, _ := IssueAPIKey(context.Background(), d, alice, "ws-bot", 0)

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())

	req := httptest.NewRequest("GET", "/x?token="+pt, nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("query-param API key: status = %d, body = %q", rr.Code, rr.Body.String())
	}
	if cap.session == nil || cap.session.DeviceID != alice {
		t.Errorf("session.DeviceID = %v, want %q", cap.session, alice)
	}
}

// --- Dispatch: failure modes ---------------------------------------------

func TestRequiredFunc_RejectsBogusAPIKey(t *testing.T) {
	// A key-shaped string that doesn't match any row → 401, generic
	// "invalid api key" body (no enumeration leak).
	d := openTestDB(t)
	bogus, _, _, _ := GenerateAPIKey()

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+bogus)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid api key") {
		t.Errorf("body = %q, want generic invalid-api-key message", rr.Body.String())
	}
	if cap.called {
		t.Error("downstream handler should not be called on auth failure")
	}
}

func TestRequiredFunc_RejectsRevokedAPIKey(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, _ := IssueAPIKey(context.Background(), d, alice, "to-revoke", 0)
	_ = RevokeAPIKey(context.Background(), d, key.ID, alice)

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("revoked key: status = %d, want 401", rr.Code)
	}
	// Body MUST collapse revoked / expired / invalid to one message
	// so an attacker can't probe "did this key ever exist?"
	if !strings.Contains(rr.Body.String(), "invalid api key") {
		t.Errorf("revoked key body = %q, want generic invalid-api-key message", rr.Body.String())
	}
}

func TestRequiredFunc_RejectsMissingAuth(t *testing.T) {
	d := openTestDB(t)
	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no auth: status = %d, want 401", rr.Code)
	}
}

// --- Dispatch ordering: API key shape wins over session lookup ----------

func TestRequiredFunc_APIKeyShapeRoutesToKeyValidator(t *testing.T) {
	// Pin that hxk_-prefixed tokens go to ValidateAPIKey, NOT
	// ValidateSession. A token that has the API-key shape but
	// happens to also appear in the session table (artificial test
	// case) should still be rejected as an invalid API key, because
	// the dispatcher routes by shape.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Plant a row in the session table whose token literally is a
	// valid-looking API key. The dispatcher should still route the
	// incoming bearer to ValidateAPIKey (which will reject it, since
	// it's not in api_key). This proves shape-based dispatch.
	keyShapedToken := APIKeyPrefix + strings.Repeat("0", APIKeyHexLen)
	_, err := d.ExecContext(context.Background(),
		`INSERT INTO session (token, device_id, created_at, expires_at, last_used_at)
		 VALUES (?, ?, 0, 0, 0)`,
		keyShapedToken, alice)
	if err != nil {
		t.Fatal(err)
	}

	cap := &captureHandler{}
	wrapped := RequiredFunc(d, cap.HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+keyShapedToken)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("shape-based dispatch broken — key-shaped token in session table validated as session; status=%d body=%q",
			rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "invalid api key") {
		t.Errorf("body = %q, want api-key rejection (proves dispatch went to ValidateAPIKey)", rr.Body.String())
	}
}

// --- Required (http.Handler variant) -------------------------------------

func TestRequired_HandlerVariantAlsoDispatches(t *testing.T) {
	// Required is the http.Handler-shaped wrapper; RequiredFunc is
	// the http.HandlerFunc-shaped one. Both must route API keys the
	// same way.
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, _, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)

	cap := &captureHandler{}
	wrapped := Required(d, http.HandlerFunc(cap.HandlerFunc()))
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Required variant: status = %d, want 200", rr.Code)
	}
	if cap.session == nil || cap.session.DeviceID != alice {
		t.Errorf("session = %+v, want device=%q", cap.session, alice)
	}
}

// --- Direct unit test on validateCredential ------------------------------

func TestValidateCredential_DispatchesByShape(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")

	// Valid session.
	sess, _ := IssueSession(context.Background(), d, alice, 3600)
	if s, err := validateCredential(context.Background(), d, sess.Token); err != nil {
		t.Errorf("session path: %v", err)
	} else if s.Token != sess.Token {
		t.Errorf("session path token mismatch: %q vs %q", s.Token, sess.Token)
	}

	// Valid API key — synthetic Session should have apikey: token prefix.
	pt, key, _ := IssueAPIKey(context.Background(), d, alice, "k", 0)
	s, err := validateCredential(context.Background(), d, pt)
	if err != nil {
		t.Errorf("apikey path: %v", err)
	}
	if s == nil {
		t.Fatal("apikey path returned nil session")
	}
	if !strings.HasPrefix(s.Token, "apikey:") {
		t.Errorf("apikey synth token = %q, want apikey: prefix", s.Token)
	}
	if !strings.HasSuffix(s.Token, key.ID) {
		t.Errorf("apikey synth token = %q, want suffix %q", s.Token, key.ID)
	}
	if s.DeviceID != alice {
		t.Errorf("apikey synth device = %q, want %q", s.DeviceID, alice)
	}
}
