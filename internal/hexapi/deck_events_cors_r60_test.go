package hexapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestDeckEvents_NoWildcardCORSHeader pins the regression fix: the
// SSE handler must NOT set Access-Control-Allow-Origin: * on its
// own. CORS belongs to cmd/hexdek-server's corsMiddleware, which
// applies an origin allowlist + Allow-Credentials: true. A per-route
// wildcard would (a) silently widen the allowlist for this one
// endpoint and (b) break browser compatibility — per CORS spec,
// `Allow-Origin: *` + `Allow-Credentials: true` is invalid and gets
// the response rejected.
func TestDeckEvents_NoWildcardCORSHeader(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/decks/{owner}/{id}/events", h.handleDeckEvents)

	// Use a srv + real GET so the handler enters its SSE loop. We
	// abort the request quickly via context cancel to read response
	// headers without waiting on the long-lived stream.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/decks/alice/aggro/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	// Simulate a malicious-origin browser to confirm the handler
	// would NOT helpfully echo or wildcard the origin back.
	req.Header.Set("Origin", "https://evil.example")

	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, gerr := http.DefaultClient.Do(req)
		if gerr != nil {
			errCh <- gerr
			return
		}
		respCh <- resp
	}()

	// Give the server a moment to write response headers, then cancel
	// so the SSE loop exits.
	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		assertNoCORSWildcard(t, resp)
		cancel()
	case err := <-errCh:
		t.Fatalf("request failed before headers: %v", err)
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("request did not return headers within 2s")
	}
}

// assertNoCORSWildcard checks the response carries no wildcard
// Allow-Origin from the handler. The handler does no CORS work itself
// — when the test wraps a corsMiddleware around the mux the global
// allowlist applies; without the wrapper (as in this test) no CORS
// headers should appear at all.
func assertNoCORSWildcard(t *testing.T, resp *http.Response) {
	t.Helper()
	got := resp.Header.Get("Access-Control-Allow-Origin")
	if got == "*" {
		t.Fatalf("handleDeckEvents set wildcard Allow-Origin — regression on the CORS-bypass fix")
	}
	// With no corsMiddleware in this test setup, no Allow-Origin
	// header should be set at all by the handler itself.
	if got != "" {
		t.Fatalf("handler set Allow-Origin=%q without corsMiddleware — handler must not do CORS work itself", got)
	}
	if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Fatalf("handler set Allow-Credentials=%q — should be middleware-only", creds)
	}
}

// TestDeckEvents_SSEHeadersIntact confirms the fix didn't take the
// rest of the SSE response shape with it — Content-Type / Cache-
// Control / Connection must still be set so EventSource clients
// actually receive a streaming response.
func TestDeckEvents_SSEHeadersIntact(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/decks/{owner}/{id}/events", h.handleDeckEvents)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/decks/bob/control/events", nil)

	respCh := make(chan *http.Response, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		respCh <- resp
	}()

	select {
	case resp := <-respCh:
		defer resp.Body.Close()
		if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
			t.Fatalf("Content-Type: want text/event-stream, got %q", ct)
		}
		if cc := resp.Header.Get("Cache-Control"); !strings.Contains(cc, "no-cache") {
			t.Fatalf("Cache-Control should include no-cache, got %q", cc)
		}
		if conn := resp.Header.Get("Connection"); !strings.EqualFold(conn, "keep-alive") {
			// Note: net/http server may strip Connection header — tolerate empty.
			if conn != "" {
				t.Fatalf("Connection: want keep-alive, got %q", conn)
			}
		}
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("request did not return headers within 2s")
	}
}

// TestDeckEvents_RejectsInvalidPath defends the path-validation
// short-circuit that protects against bucket-key smuggling via
// "../" or slashed owner/id. Pre-existed but worth pinning along
// with the CORS fix so any future refactor that touches the header
// block can't accidentally drop the validation too.
func TestDeckEvents_RejectsInvalidPath(t *testing.T) {
	h := &Handler{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/decks/{owner}/{id}/events", h.handleDeckEvents)

	for _, badPath := range []string{
		"/api/decks/..%2Falice/aggro/events",
		"/api/decks/alice/..%2Fevents",
		"/api/decks/alice/aggro%2Fevil/events",
	} {
		req := httptest.NewRequest("GET", badPath, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			t.Fatalf("path %q should be rejected, got 200", badPath)
		}
	}
}
