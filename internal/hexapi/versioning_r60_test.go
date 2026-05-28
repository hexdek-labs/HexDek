package hexapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// versioning_r60_test pins the VersioningMiddleware decision branches:
// (1) /api/v1/* requests get the /v1 prefix stripped before the
// underlying mux sees them and gain X-HexDek-API-Version: v1;
// (2) /api/* legacy requests reach the same handler but gain
// Deprecation/Sunset/Link/X-HexDek-API-Version: legacy headers;
// (3) non-/api paths pass through untouched. Plus defaultSuccessorPath's
// /api/ → /api/v1/ rewrite (and the defensive no-double-version guard).

// echoHandler is a stub next-handler that records what URL.Path it
// observed — used by the middleware tests to assert the /v1 strip
// actually took effect before the handler saw the request.
type echoHandler struct {
	gotPath  string
	gotQuery string
}

func (e *echoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.gotPath = r.URL.Path
	e.gotQuery = r.URL.RawQuery
	w.WriteHeader(http.StatusOK)
}

// TestVersioningMiddleware_V1StripsPrefix verifies the canonical v1
// path: /api/v1/decks/123 reaches the next handler as /api/decks/123,
// and the response carries the v1 marker header.
func TestVersioningMiddleware_V1StripsPrefix(t *testing.T) {
	next := &echoHandler{}
	mw := VersioningMiddleware(next, VersioningOpts{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decks/123", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if next.gotPath != "/api/decks/123" {
		t.Errorf("next-handler saw path %q, want %q (v1 segment should be stripped)",
			next.gotPath, "/api/decks/123")
	}
	if got := rec.Header().Get("X-HexDek-API-Version"); got != "v1" {
		t.Errorf("X-HexDek-API-Version = %q, want v1", got)
	}
	if got := rec.Header().Get("Deprecation"); got != "" {
		t.Errorf("v1 path should NOT carry Deprecation header, got %q", got)
	}
}

// TestVersioningMiddleware_LegacyAttachesDeprecation verifies the
// /api/* legacy path: same handler reached unchanged, but the response
// carries Deprecation/Sunset/Link/legacy-marker headers so clients
// can detect the migration is coming.
func TestVersioningMiddleware_LegacyAttachesDeprecation(t *testing.T) {
	next := &echoHandler{}
	mw := VersioningMiddleware(next, VersioningOpts{})

	req := httptest.NewRequest(http.MethodGet, "/api/decks/123", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if next.gotPath != "/api/decks/123" {
		t.Errorf("legacy path should reach handler unchanged, got %q", next.gotPath)
	}
	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Errorf("Deprecation header = %q, want 'true'", got)
	}
	if rec.Header().Get("Sunset") == "" {
		t.Errorf("Sunset header missing")
	}
	link := rec.Header().Get("Link")
	if !strings.Contains(link, `</api/v1/decks/123>`) || !strings.Contains(link, `rel="successor-version"`) {
		t.Errorf("Link header malformed: %q", link)
	}
	if got := rec.Header().Get("X-HexDek-API-Version"); got != "legacy" {
		t.Errorf("X-HexDek-API-Version = %q, want 'legacy'", got)
	}
}

// TestVersioningMiddleware_NonAPIPassesThrough verifies the pass-
// through branch: paths outside /api/ (websockets, GraphQL, share
// pages) reach the handler untouched, with no version headers
// attached. The middleware must not pollute non-REST endpoints.
func TestVersioningMiddleware_NonAPIPassesThrough(t *testing.T) {
	cases := []string{"/ws/lobby", "/graphql", "/decks/atraxa", "/share/abc123", "/spectate", "/leaderboard", "/"}
	for _, path := range cases {
		next := &echoHandler{}
		mw := VersioningMiddleware(next, VersioningOpts{})
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mw.ServeHTTP(rec, req)

		if next.gotPath != path {
			t.Errorf("%s: path mutated to %q", path, next.gotPath)
		}
		if rec.Header().Get("Deprecation") != "" {
			t.Errorf("%s: Deprecation header should not be set on non-/api/ paths", path)
		}
		if rec.Header().Get("X-HexDek-API-Version") != "" {
			t.Errorf("%s: version header should not be set on non-/api/ paths", path)
		}
	}
}

// TestVersioningMiddleware_CustomSunsetUsed verifies the
// VersioningOpts.SunsetAt override path: a non-zero SunsetAt overrides
// the package default for the Sunset header value.
func TestVersioningMiddleware_CustomSunsetUsed(t *testing.T) {
	customSunset := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	next := &echoHandler{}
	mw := VersioningMiddleware(next, VersioningOpts{SunsetAt: customSunset})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	got := rec.Header().Get("Sunset")
	want := customSunset.Format(http.TimeFormat)
	if got != want {
		t.Errorf("Sunset header = %q, want %q (custom override)", got, want)
	}
}

// TestVersioningMiddleware_CustomSuccessorPathUsed verifies the
// SuccessorPath override: when set, the Link header uses the custom
// rewriter instead of defaultSuccessorPath.
func TestVersioningMiddleware_CustomSuccessorPathUsed(t *testing.T) {
	next := &echoHandler{}
	mw := VersioningMiddleware(next, VersioningOpts{
		SuccessorPath: func(legacy string) string { return "/v2" + legacy },
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	link := rec.Header().Get("Link")
	if !strings.Contains(link, "</v2/api/test>") {
		t.Errorf("custom SuccessorPath not used, Link = %q", link)
	}
}

// TestVersioningMiddleware_QueryStringPreserved pins that the /v1
// strip rewrites only the path, not the query string. A request like
// /api/v1/decks?q=atraxa must reach the handler with the q parameter
// intact.
func TestVersioningMiddleware_QueryStringPreserved(t *testing.T) {
	next := &echoHandler{}
	mw := VersioningMiddleware(next, VersioningOpts{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/decks?q=atraxa&limit=10", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if next.gotPath != "/api/decks" {
		t.Errorf("path = %q, want %q", next.gotPath, "/api/decks")
	}
	if !strings.Contains(next.gotQuery, "q=atraxa") || !strings.Contains(next.gotQuery, "limit=10") {
		t.Errorf("query string lost: %q", next.gotQuery)
	}
}

// TestDefaultSuccessorPath_RewritesAPIPath pins the basic rewrite:
// /api/decks → /api/v1/decks.
func TestDefaultSuccessorPath_RewritesAPIPath(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"/api/decks", "/api/v1/decks"},
		{"/api/decks/123", "/api/v1/decks/123"},
		{"/api/csrf", "/api/v1/csrf"},
	}
	for _, tc := range cases {
		if got := defaultSuccessorPath(tc.input); got != tc.want {
			t.Errorf("defaultSuccessorPath(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// TestDefaultSuccessorPath_NoDoubleVersion pins the defensive guard:
// passing a path that's already /api/v1/* doesn't double-version it
// to /api/v1/v1/*. Defends against a caller accidentally feeding the
// already-rewritten path back through.
func TestDefaultSuccessorPath_NoDoubleVersion(t *testing.T) {
	cases := []string{"/api/v1/decks", "/api/v1", "/api/v1/csrf"}
	for _, input := range cases {
		if got := defaultSuccessorPath(input); got != input {
			t.Errorf("defaultSuccessorPath(%q) double-versioned to %q", input, got)
		}
	}
}

// TestDefaultSuccessorPath_NonAPIReturnsUnchanged pins the
// "shouldn't happen" guard: a non-/api/ path returns unchanged
// rather than getting incorrectly prefixed. The middleware never
// calls this on non-/api/ paths but the function is defensively
// callable.
func TestDefaultSuccessorPath_NonAPIReturnsUnchanged(t *testing.T) {
	if got := defaultSuccessorPath("/ws/lobby"); got != "/ws/lobby" {
		t.Errorf("non-/api/ path mutated: got %q", got)
	}
}
