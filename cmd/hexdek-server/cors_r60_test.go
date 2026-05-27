package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// inner is a no-op handler used to drive corsMiddleware in isolation —
// returns 200 with a short body so we can confirm the middleware
// reached `next` (or didn't, for the OPTIONS preflight short-circuit).
var inner = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

// TestCORS_AllowedOriginEchoed pins the core allowlist contract:
// a request from a configured origin (production frontend) gets its
// Origin echoed back with Allow-Credentials:true.
func TestCORS_AllowedOriginEchoed(t *testing.T) {
	h := corsMiddleware(inner)
	for _, origin := range []string{
		"https://hexdek.bluefroganalytics.com",
		"http://localhost:5173",
		"http://localhost:4173",
		"http://127.0.0.1:5173",
	} {
		req := httptest.NewRequest("GET", "/api/anything", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("origin %q: want echo %q, got %q", origin, origin, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("origin %q: want Allow-Credentials=true, got %q", origin, got)
		}
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("origin %q: want Vary: Origin, got %q", origin, got)
		}
	}
}

// TestCORS_DisallowedOriginNotEchoed confirms an unknown origin gets
// NO Allow-Origin header — browsers will block the cross-site
// response. The middleware must NOT fall back to wildcard or echo
// the unknown origin.
func TestCORS_DisallowedOriginNotEchoed(t *testing.T) {
	h := corsMiddleware(inner)
	for _, origin := range []string{
		"https://evil.example",
		"https://hexdek.com.evil.example", // suffix-attack variant
		"http://localhost:5173.evil.example",
	} {
		req := httptest.NewRequest("GET", "/api/anything", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)

		if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("origin %q: must NOT be echoed, got %q", origin, got)
		}
		if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("origin %q: must NOT carry Allow-Credentials, got %q", origin, got)
		}
		// Vary: Origin still set so caches partition correctly.
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("origin %q: want Vary: Origin regardless of allowlist, got %q", origin, got)
		}
	}
}

// TestCORS_NoWildcardEver is the regression sentinel: under no
// origin (allowed, disallowed, missing, LAN) should the middleware
// emit `Allow-Origin: *`. The deck-events handler used to set this
// inline; the fix moved CORS fully into the middleware and the
// middleware itself must never wildcard.
func TestCORS_NoWildcardEver(t *testing.T) {
	h := corsMiddleware(inner)
	for _, origin := range []string{
		"", // no Origin header
		"https://hexdek.bluefroganalytics.com",
		"https://evil.example",
		"http://localhost:5173",
		"http://192.168.1.42:5173",
	} {
		req := httptest.NewRequest("GET", "/api/anything", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got == "*" {
			t.Fatalf("origin %q: middleware emitted wildcard Allow-Origin", origin)
		}
	}
}

// TestCORS_LANPrefixAllowed pins the dev-environment carve-out for
// LAN testing (phones / tablets on the local network connecting to
// the dev server). Future tightening of this prefix should land with
// a documented production-vs-dev split.
func TestCORS_LANPrefixAllowed(t *testing.T) {
	h := corsMiddleware(inner)
	for _, origin := range []string{
		"http://192.168.1.42:5173",
		"http://192.168.1.207:8090",
	} {
		req := httptest.NewRequest("GET", "/api/anything", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("LAN origin %q: want echo, got %q", origin, got)
		}
	}
}

// TestCORS_OPTIONSPreflightShortCircuits pins the preflight contract:
// OPTIONS returns 204 with CORS headers set and does NOT call into
// the next handler. Verifies the inner handler did not run by
// checking the body is empty (the inner writes "ok" on GET).
func TestCORS_OPTIONSPreflightShortCircuits(t *testing.T) {
	h := corsMiddleware(inner)
	req := httptest.NewRequest("OPTIONS", "/api/anything", nil)
	req.Header.Set("Origin", "https://hexdek.bluefroganalytics.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight: want 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("OPTIONS preflight reached inner handler — body=%q (must short-circuit)", w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("OPTIONS preflight missing Allow-Methods header")
	}
	if got := w.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("OPTIONS preflight missing Allow-Headers header")
	}
}

// TestCORS_AlwaysSetsVary defends the cache-correctness contract:
// Vary: Origin must always be present because Allow-Origin varies
// per request. Without it, a shared cache (CDN, browser, intermediate
// proxy) could serve a response cached for origin A to a request
// from origin B, defeating the allowlist.
func TestCORS_AlwaysSetsVary(t *testing.T) {
	h := corsMiddleware(inner)
	for _, origin := range []string{"", "https://hexdek.bluefroganalytics.com", "https://evil.example"} {
		req := httptest.NewRequest("GET", "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if got := w.Header().Get("Vary"); got != "Origin" {
			t.Errorf("origin %q: Vary must always be Origin, got %q", origin, got)
		}
	}
}

// TestIsOriginAllowed_HostnameNotPrefix is the unit-level pin for
// the prefix-match vulnerability: parsing the origin into host
// components prevents an attacker-controlled subdomain
// ("localhost:5173.evil.example") from matching the dev-host rule.
// Catches the exact regression class the SECOND vulnerability in
// this PR was about.
func TestIsOriginAllowed_HostnameNotPrefix(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
		why    string
	}{
		// Allowed
		{"https://hexdek.bluefroganalytics.com", true, "production frontend"},
		{"http://localhost:5173", true, "dev vite"},
		{"http://localhost:4173", true, "dev vite preview"},
		{"http://localhost:9999", true, "any localhost port"},
		{"http://127.0.0.1:5173", true, "loopback ip"},
		{"http://192.168.1.42:5173", true, "LAN /24"},
		{"http://192.168.1.207:8090", true, "DARKSTAR LAN"},

		// Disallowed — the regression cases
		{"http://localhost:5173.evil.example", false, "subdomain attack on localhost prefix"},
		{"http://localhost.evil.example", false, "subdomain attack without port"},
		{"http://192.168.1.1.evil.example", false, "subdomain attack on LAN prefix"},
		{"https://hexdek.bluefroganalytics.com.evil.example", false, "suffix attack on prod host"},

		// Disallowed — generally
		{"", false, "empty origin"},
		{"https://evil.example", false, "unknown host"},
		{"http://192.168.2.1", false, "outside the /24"},
		{"http://10.0.0.1", false, "different RFC1918 range"},

		// Disallowed schemes
		{"file:///etc/passwd", false, "file scheme"},
		{"chrome-extension://abc", false, "extension scheme"},
		{"data:text/html,<script>", false, "data URI"},

		// Malformed — must not panic, must return false
		{"http://[::1", false, "unparseable url"},
		{"not a url at all", false, "garbage origin"},
	}
	for _, tc := range cases {
		got := isOriginAllowed(tc.origin)
		if got != tc.want {
			t.Errorf("isOriginAllowed(%q) = %v, want %v (%s)", tc.origin, got, tc.want, tc.why)
		}
	}
}

// TestCORS_ExposeRequestID confirms X-Request-Id is exposed to
// browser clients so error-report widgets can surface the id. This
// header is non-CORS-safelisted by default and would be hidden from
// fetch() without explicit Expose-Headers.
func TestCORS_ExposeRequestID(t *testing.T) {
	h := corsMiddleware(inner)
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://hexdek.bluefroganalytics.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-Id" {
		t.Fatalf("Expose-Headers: want X-Request-Id, got %q", got)
	}
}
