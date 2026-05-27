package hexapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// csrfGatedRoutes is the authoritative list of routes wrapped in
// RequireCSRF when CSRFStore is non-nil. Pinning this set in a test
// lets a reviewer add a route to the wrapping below + add it here,
// and instantly see the contract change in code review. Drifts (a
// route silently dropped from the gating) fire TestCSRFCoverage_
// AllExpectedRoutesAreGated below.
//
// Format: method + " " + path-template. Path-template uses the same
// {placeholder} shape as Go 1.22 ServeMux patterns.
var csrfGatedRoutes = []string{
	"POST /api/decks",
	"POST /api/decks/import",
	"PUT /api/decks/{owner}/{id}",
	"PATCH /api/decks/{owner}/{id}",
	"DELETE /api/decks/{owner}/{id}",
	"POST /api/decks/{owner}/{id}/analyze",
	"POST /api/decks/{owner}/{id}/clone",
	"POST /api/decks/{owner}/{id}/fork",
	"POST /api/decks/{owner}/{id}/archetype-feedback",
	"POST /api/import/moxfield",
	"POST /api/import/archidekt",
}

// TestCSRFCoverage_AllExpectedRoutesAreGated pins the full set of
// CSRF-gated mutating routes. When CSRFStore is set, every route in
// csrfGatedRoutes must reject a request that lacks the X-CSRF-Token
// header with a 403. Failures here mean someone either added a new
// mutating route without CSRF or dropped CSRF from an existing one
// — both are security regressions and should be caught at review.
func TestCSRFCoverage_AllExpectedRoutesAreGated(t *testing.T) {
	secret, err := RandomCSRFSecret()
	if err != nil {
		t.Fatalf("seed secret: %v", err)
	}
	h := &Handler{CSRFStore: NewCSRFStore(secret, 0)}
	mux := http.NewServeMux()
	h.Register(mux)

	for _, route := range csrfGatedRoutes {
		parts := strings.SplitN(route, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("malformed route in csrfGatedRoutes: %q", route)
		}
		method, pathTemplate := parts[0], parts[1]
		// Sub in concrete values for the {owner}/{id} placeholders so
		// the request actually routes.
		path := strings.ReplaceAll(pathTemplate, "{owner}", "alice")
		path = strings.ReplaceAll(path, "{id}", "aggro")

		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		// Deliberately omit X-CSRF-Token.
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s: expected 403 without CSRF token, got %d (body=%q)",
				route, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "csrf") {
			t.Errorf("%s: 403 body should mention csrf, got %q", route, w.Body.String())
		}
	}
}

// TestCSRFCoverage_GatedRoutesPassWithValidToken confirms the
// positive path: requests carrying a valid X-CSRF-Token reach the
// downstream handler. We can't easily run every handler end-to-end
// (each needs different DB/Showmatch fixtures), so this test asserts
// the response is NOT a CSRF 403 — anything else (200, 400 for empty
// body, 404, 503) means the gate let the request through.
func TestCSRFCoverage_GatedRoutesPassWithValidToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, 0)
	h := &Handler{CSRFStore: store}
	mux := http.NewServeMux()
	h.Register(mux)

	tok, _, err := store.IssueToken()
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	for _, route := range csrfGatedRoutes {
		parts := strings.SplitN(route, " ", 2)
		method, pathTemplate := parts[0], parts[1]
		path := strings.ReplaceAll(pathTemplate, "{owner}", "alice")
		path = strings.ReplaceAll(path, "{id}", "aggro")

		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(CSRFHeader, tok)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
			t.Errorf("%s: valid CSRF token was rejected (403 csrf), got body=%q", route, w.Body.String())
		}
	}
}

// TestCSRFCoverage_NilStoreIsPassThrough pins the backwards-compat
// contract: when CSRFStore is nil, no enforcement happens — older
// binaries / frontends that don't issue tokens keep working. Defends
// against a refactor that makes CSRFStore unconditionally required,
// which would break every existing deployment on upgrade.
func TestCSRFCoverage_NilStoreIsPassThrough(t *testing.T) {
	h := &Handler{} // no CSRFStore
	mux := http.NewServeMux()
	h.Register(mux)

	// Pick one representative gated route and confirm a no-token
	// request is NOT 403'd (gets through to the handler, which will
	// emit some other error for the empty/incomplete request).
	req := httptest.NewRequest("DELETE", "/api/decks/alice/aggro", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
		t.Fatalf("nil CSRFStore must not enforce — got 403 csrf, body=%q", w.Body.String())
	}
}

// TestCSRFCoverage_GETEndpointsNotGated is the negative-coverage
// check: read endpoints must NOT require CSRF (GETs are
// idempotent and not part of the CSRF threat model). A regression
// that wrapped GETs in RequireCSRF would break every read for
// clients that don't have a token cached.
func TestCSRFCoverage_GETEndpointsNotGated(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h := &Handler{CSRFStore: NewCSRFStore(secret, 0)}
	mux := http.NewServeMux()
	h.Register(mux)

	// Sample a few GET routes that should NEVER be CSRF-gated.
	for _, route := range []string{
		"GET /api/decks",
		"GET /api/decks/alice/aggro",
		"GET /api/meta",
		"GET /api/csrf", // token issuance itself can't require a token
	} {
		parts := strings.SplitN(route, " ", 2)
		req := httptest.NewRequest(parts[0], parts[1], nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
			t.Errorf("%s: GET endpoint should not require CSRF, got 403 csrf", route)
		}
	}
}

// TestCSRFCoverage_PublicMutatingEndpointsIntentionallyUngated
// documents (as a passing test) that some POST endpoints are
// intentionally NOT CSRF-gated and explains why. Acts as an explicit
// allowlist of "we considered this and chose not to" so a reviewer
// asking "why isn't /api/feedback CSRF-gated?" sees the answer
// here instead of git-blaming the omission.
func TestCSRFCoverage_PublicMutatingEndpointsIntentionallyUngated(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h := &Handler{CSRFStore: NewCSRFStore(secret, 0)}
	mux := http.NewServeMux()
	h.Register(mux)

	cases := []struct {
		route  string
		reason string
	}{
		{"POST /api/feedback", "public anonymous submit form; rate-limited via FeedbackLimiter"},
		{"POST /api/kofi/webhook", "external service webhook; carries its own HMAC signature"},
		{"POST /api/telemetry/pageview", "beacon-style telemetry; no owner-scoped state mutation"},
		{"POST /api/telemetry/stitch", "beacon-style telemetry; no owner-scoped state mutation"},
	}
	for _, tc := range cases {
		parts := strings.SplitN(tc.route, " ", 2)
		req := httptest.NewRequest(parts[0], parts[1], strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		// We expect any status OTHER than 403-csrf. A 400/422/etc is
		// fine — the handler ran and complained about something else.
		if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
			t.Errorf("%s should be ungated (%s) but returned 403 csrf", tc.route, tc.reason)
		}
	}
}
