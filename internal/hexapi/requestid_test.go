package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRequestIDMiddleware_GeneratesWhenAbsent verifies that the
// middleware generates a fresh ID when the caller did not supply one.
// The ID must be set on the response header BEFORE the wrapped
// handler runs (so write helpers can read it back) AND be available
// via RequestIDFromContext.
func TestRequestIDMiddleware_GeneratesWhenAbsent(t *testing.T) {
	var ctxID string
	var headerAtHandlerTime string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = RequestIDFromContext(r.Context())
		headerAtHandlerTime = w.Header().Get(RequestIDHeader)
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	respID := rr.Header().Get(RequestIDHeader)
	if respID == "" {
		t.Fatalf("response X-Request-Id was not set by middleware")
	}
	if len(respID) != 32 {
		t.Errorf("generated ID should be 32 hex chars (128 bits), got %d (%q)", len(respID), respID)
	}
	if ctxID != respID {
		t.Errorf("context ID and response header diverged: ctx=%q hdr=%q", ctxID, respID)
	}
	if headerAtHandlerTime != respID {
		t.Errorf("response X-Request-Id must be visible to handler BEFORE it writes; ctx=%q at-handler=%q", respID, headerAtHandlerTime)
	}
}

// TestRequestIDMiddleware_PreservesCallerSupplied pins the
// round-trip: if the caller (load balancer, frontend fetch wrapper,
// test harness) supplied an X-Request-Id, the middleware must echo it
// verbatim instead of overwriting with a fresh value.
func TestRequestIDMiddleware_PreservesCallerSupplied(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.Header.Set(RequestIDHeader, "caller-supplied-trace-123")

	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	if got := rr.Header().Get(RequestIDHeader); got != "caller-supplied-trace-123" {
		t.Errorf("caller-supplied X-Request-Id not preserved: want caller-supplied-trace-123, got %q", got)
	}
}

// TestRequestIDMiddleware_RejectsBogusCallerSupplied pins the
// hardening: callers cannot inject unbounded or control-character
// strings into our trace logs. Anything outside the plausible range
// gets replaced with a fresh ID.
func TestRequestIDMiddleware_RejectsBogusCallerSupplied(t *testing.T) {
	cases := []struct {
		name string
		hdr  string
	}{
		{"too short", "abc"},
		{"too long", strings.Repeat("x", 200)},
		{"control character", "abcdef\x00gh"},
		{"whitespace", "abcd efgh"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			req.Header.Set(RequestIDHeader, tc.hdr)
			RequestIDMiddleware(handler).ServeHTTP(rr, req)
			got := rr.Header().Get(RequestIDHeader)
			if got == tc.hdr {
				t.Errorf("bogus caller-supplied ID %q was echoed back instead of replaced", tc.hdr)
			}
			if len(got) != 32 {
				t.Errorf("replacement ID should be 32 hex chars, got %d (%q)", len(got), got)
			}
		})
	}
}

// TestRequestIDMiddleware_IntegratesWithWriteError pins the full
// round-trip: middleware-wrapped handler emits an error, the body's
// request_id field matches the response header, and both match the
// context value the handler saw. This is the end-to-end contract
// callers depend on for trace-back from a browser error to a server
// log line.
func TestRequestIDMiddleware_IntegratesWithWriteError(t *testing.T) {
	var ctxID string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = RequestIDFromContext(r.Context())
		writeError(w, http.StatusForbidden, "no access")
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	respHdrID := rr.Header().Get(RequestIDHeader)
	if respHdrID == "" {
		t.Fatal("middleware did not set X-Request-Id on response")
	}
	if ctxID != respHdrID {
		t.Errorf("context vs header diverge: ctx=%q hdr=%q", ctxID, respHdrID)
	}

	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
	}
	if body.RequestID != respHdrID {
		t.Errorf("body.request_id %q did not match response header X-Request-Id %q", body.RequestID, respHdrID)
	}
	if body.Error.Code != "forbidden" || body.Error.Message != "no access" {
		t.Errorf("body.error: want {forbidden, no access}, got %+v", body.Error)
	}
}

// TestRequestIDFromContext_EmptyWithoutMiddleware verifies that
// callers that bypass the middleware get the empty string, not a
// panic. (The helper has to be safe in unit tests that construct a
// bare context.)
func TestRequestIDFromContext_EmptyWithoutMiddleware(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	if got := RequestIDFromContext(req.Context()); got != "" {
		t.Errorf("want empty string for unwrapped context, got %q", got)
	}
}

// TestIsPlausibleRequestID pins the exact accept/reject set so the
// middleware's substitution behavior stays predictable.
func TestIsPlausibleRequestID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"short", "abc", false},
		{"min length", "abcdefgh", true},
		{"max length", strings.Repeat("a", 128), true},
		{"over max", strings.Repeat("a", 129), false},
		{"hex", "deadbeefdeadbeefdeadbeefdeadbeef", true},
		{"with hyphens", "trace-123-deadbeef", true},
		{"with space", "abcd efgh", false},
		{"with null", "abcdef\x00gh", false},
		{"with newline", "abcdef\ngh", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isPlausibleRequestID(tc.in); got != tc.want {
				t.Errorf("isPlausibleRequestID(%q): want %v, got %v", tc.in, tc.want, got)
			}
		})
	}
}
