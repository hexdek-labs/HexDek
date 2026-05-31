package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// middleware_envelope_r60_test.go — pins that Required / RequiredFunc
// emit a 401 in the unified ErrorResponse envelope shape (PR #778),
// NOT plain-text via http.Error. Pre-r60 the middleware called
// http.Error(w, msg, 401) which broke the contract for every protected
// endpoint's auth-failure path: clients parsing `error.code` got a
// plain string instead, and the X-Content-Type-Options nosniff +
// request_id round-trip both went missing.
//
// Wire shape pinned here (must stay bit-identical to hexapi.ErrorResponse):
//
//	{
//	  "error":      {"code": "<slug>", "message": "<text>"},
//	  "request_id": "<echoed from X-Request-Id header if set>",
//	  "status":     401
//	}
//
// Plus headers: Content-Type: application/json, X-Content-Type-Options: nosniff.

// envelopeOf decodes the body as the unified shape and fails fast if
// it doesn't parse. Centralized so per-branch tests stay focused.
func envelopeOf(t *testing.T, rr *httptest.ResponseRecorder) errorResponse {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}
	if nosniff := rr.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", nosniff)
	}
	var body errorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("body did not decode as errorResponse: %v (raw=%q)", err, rr.Body.String())
	}
	if body.Status != http.StatusUnauthorized {
		t.Errorf("body.status: want 401, got %d", body.Status)
	}
	return body
}

// TestRequired_MissingToken_EnvelopeShape pins the no-credentials path:
// extractToken returns "", which ValidateSession rejects as
// ErrInvalidToken → code "invalid_token".
func TestRequired_MissingToken_EnvelopeShape(t *testing.T) {
	d := openTestDB(t)
	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	body := envelopeOf(t, rr)
	if body.Error.Code != "invalid_token" {
		t.Errorf("error.code = %q, want invalid_token", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "invalid or unknown session token") {
		t.Errorf("error.message = %q, want the canonical invalid-token text", body.Error.Message)
	}
}

// TestRequired_ExpiredSession_EnvelopeShape pins the expired-session
// path: a real session row whose ExpiresAt is in the past yields
// ErrSessionExpired → code "session_expired".
func TestRequired_ExpiredSession_EnvelopeShape(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	// Issue then expire by hand — backdate ExpiresAt to t=1 (well in
	// the past) so ValidateSession returns ErrSessionExpired without
	// waiting on real wall time.
	sess, err := IssueSession(context.Background(), d, alice, 3600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.ExecContext(context.Background(),
		`UPDATE session SET expires_at = 1 WHERE token = ?`, sess.Token); err != nil {
		t.Fatal(err)
	}

	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	body := envelopeOf(t, rr)
	if body.Error.Code != "session_expired" {
		t.Errorf("error.code = %q, want session_expired", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "session token expired") {
		t.Errorf("error.message = %q, want session-expired text", body.Error.Message)
	}
}

// TestRequired_BogusAPIKey_EnvelopeShape pins the API-key failure path:
// any hxk_-shaped token that isn't in the DB collapses to a single
// "invalid_api_key" slug — the same enumeration-leak guard that
// previously collapsed the message text now also collapses the code,
// so an attacker can't distinguish "key revoked" from "key never
// existed" via code-switching either.
func TestRequired_BogusAPIKey_EnvelopeShape(t *testing.T) {
	d := openTestDB(t)
	bogus, _, _, _ := GenerateAPIKey()
	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+bogus)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	body := envelopeOf(t, rr)
	if body.Error.Code != "invalid_api_key" {
		t.Errorf("error.code = %q, want invalid_api_key", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "invalid api key") {
		t.Errorf("error.message = %q, want invalid-api-key text", body.Error.Message)
	}
}

// TestRequired_RevokedAPIKey_CodeCollapsesToInvalidAPIKey is the
// enumeration-leak guard at the code-slug level. Revoked / expired /
// invalid all MUST share one slug — distinct slugs would let a probing
// client distinguish "did this key ever exist?" the same way distinct
// messages would.
func TestRequired_RevokedAPIKey_CodeCollapsesToInvalidAPIKey(t *testing.T) {
	d := openTestDB(t)
	alice := seedDevice(t, d, "device_alice", "Alice")
	pt, key, _ := IssueAPIKey(context.Background(), d, alice, "to-revoke", 0)
	_ = RevokeAPIKey(context.Background(), d, key.ID, alice)

	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+pt)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	body := envelopeOf(t, rr)
	if body.Error.Code != "invalid_api_key" {
		t.Errorf("revoked-key error.code = %q, want invalid_api_key (must collapse with bogus key to defeat enumeration)", body.Error.Code)
	}
}

// TestRequired_RequestIDRoundTrip pins that when an upstream
// RequestIDMiddleware has stamped X-Request-Id onto the response writer
// (the standard hexapi composition), our 401 body echoes the same ID in
// the `request_id` field — making auth failures traceable end-to-end
// the way every other unified-envelope error already is.
func TestRequired_RequestIDRoundTrip(t *testing.T) {
	d := openTestDB(t)
	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())

	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	// Simulate upstream RequestIDMiddleware stamping the header on the
	// response writer before our middleware runs.
	rr.Header().Set(requestIDHeader, "req-12345-abc")
	wrapped(rr, req)

	body := envelopeOf(t, rr)
	if body.RequestID != "req-12345-abc" {
		t.Errorf("request_id = %q, want %q (upstream RequestIDMiddleware echo broken)", body.RequestID, "req-12345-abc")
	}
}

// TestRequired_NoRequestID_OmitsField pins the `omitempty` behavior:
// when no upstream middleware stamped X-Request-Id, the field is
// omitted from the wire body (not emitted as `"request_id": ""`).
// Mirrors hexapi.ErrorResponse's contract — the field is optional.
func TestRequired_NoRequestID_OmitsField(t *testing.T) {
	d := openTestDB(t)
	wrapped := RequiredFunc(d, (&captureHandler{}).HandlerFunc())
	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	wrapped(rr, req)

	raw := rr.Body.String()
	if strings.Contains(raw, `"request_id"`) {
		t.Errorf("expected request_id to be omitted, but body contains it: %s", raw)
	}
}

// TestRequired_HandlerVariant_AlsoEmitsEnvelope pins that the
// http.Handler-shaped wrapper Required (not just the HandlerFunc-shaped
// RequiredFunc) also emits the unified envelope. Both must behave
// identically — they're the only two auth choke points.
func TestRequired_HandlerVariant_AlsoEmitsEnvelope(t *testing.T) {
	d := openTestDB(t)
	wrapped := Required(d, http.HandlerFunc((&captureHandler{}).HandlerFunc()))
	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	body := envelopeOf(t, rr)
	if body.Error.Code != "invalid_token" {
		t.Errorf("Required (Handler variant) error.code = %q, want invalid_token", body.Error.Code)
	}
}

// TestAuthErrorCodeMessage_AllSentinels pins the slug taxonomy directly
// at the function boundary — a unit-level guard against future
// refactors silently breaking the contract before any wire-level
// integration test catches it.
func TestAuthErrorCodeMessage_AllSentinels(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantCode string
		wantMsgContains string
	}{
		{"invalid token", ErrInvalidToken, "invalid_token", "invalid or unknown session token"},
		{"expired session", ErrSessionExpired, "session_expired", "session token expired"},
		{"invalid api key", ErrInvalidAPIKey, "invalid_api_key", "invalid api key"},
		{"expired api key", ErrAPIKeyExpired, "invalid_api_key", "invalid api key"},
		{"revoked api key", ErrAPIKeyRevoked, "invalid_api_key", "invalid api key"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			code, msg := authErrorCodeMessage(tc.err)
			if code != tc.wantCode {
				t.Errorf("code = %q, want %q", code, tc.wantCode)
			}
			if !strings.Contains(msg, tc.wantMsgContains) {
				t.Errorf("message = %q, want substring %q", msg, tc.wantMsgContains)
			}
		})
	}
}
