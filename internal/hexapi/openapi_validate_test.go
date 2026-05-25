package hexapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// r60 — runtime OpenAPI validation middleware tests.
//
// These tests load the canonical docs/hexapi-openapi.yaml at startup,
// build an OpenAPIValidator, and exercise the four orthogonal modes
// the middleware supports (strict-request, log-only-request,
// strict-response, log-only-response) plus the skip-path bypass.
//
// Coverage focuses on the middleware's *own* behaviour. Schema
// fidelity (i.e. "does endpoint X really emit the documented shape")
// is the job of the per-endpoint handler tests + the drift-detector
// in openapi_coverage_r60_test.go; we don't try to re-prove it
// here.

// specPathForTests returns the path to docs/hexapi-openapi.yaml as
// resolved from any package CWD inside this repo.
func specPathForTests(t *testing.T) string {
	t.Helper()
	root := findRepoRoot(t)
	return filepath.Join(root, "docs", "hexapi-openapi.yaml")
}

// recordingLogf captures Logf calls so tests can assert on what the
// validator decided to warn about. Thread-safe — the validator
// itself doesn't call Logf concurrently for one request, but
// multiple requests in a test could overlap.
type recordingLogf struct {
	mu    sync.Mutex
	lines []string
}

func (r *recordingLogf) f(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, fmt.Sprintf(format, args...))
}

func (r *recordingLogf) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// -------------------------------------------------------------------
// Spec-level smoke tests
// -------------------------------------------------------------------

func TestOpenAPIValidate_SpecLoadsAndIsValid(t *testing.T) {
	doc, err := ValidateLoadedSpec(specPathForTests(t))
	if err != nil {
		t.Fatalf("spec failed self-validation: %v", err)
	}
	if doc == nil {
		t.Fatal("loaded spec is nil")
	}
	if got := doc.OpenAPI; !strings.HasPrefix(got, "3.") {
		t.Errorf("openapi field: want 3.x, got %q", got)
	}
	if len(doc.Paths.Map()) == 0 {
		t.Error("spec has no paths declared")
	}
	if doc.Components == nil || len(doc.Components.Schemas) == 0 {
		t.Error("spec has no components.schemas")
	}
}

func TestOpenAPIValidate_NewValidator_RequiresSpecPath(t *testing.T) {
	if _, err := NewOpenAPIValidator(OpenAPIValidatorOpts{}); err == nil {
		t.Fatal("expected error for empty SpecPath, got nil")
	}
}

func TestOpenAPIValidate_NewValidator_BadPath(t *testing.T) {
	if _, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath: "/no/such/path/hexapi.yaml",
	}); err == nil {
		t.Fatal("expected error for missing spec file, got nil")
	}
}

// -------------------------------------------------------------------
// Skip-path bypass
// -------------------------------------------------------------------

func TestOpenAPIValidate_SkipPaths_BypassMiddleware(t *testing.T) {
	rec := &recordingLogf{}
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:              specPathForTests(t),
		RejectInvalidRequests: true,
		Logf:                  rec.f,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}

	cases := []struct {
		name string
		path string
	}{
		{"websocket live spectator", "/ws/live"},
		{"websocket spectate room", "/ws/spectate/abc"},
		{"contrib socket", "/api/contrib/connect"},
		{"deck SSE", "/api/decks/alice/korvold/events"},
		{"tournament SSE", "/api/tournaments/alice/korvold/events"},
		{"share page", "/decks/alice/korvold"},
		{"card art binary", "/api/card-art/Sol%20Ring"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			handlerCalled := false
			h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if !handlerCalled {
				t.Errorf("handler not reached — middleware did not skip %s", tc.path)
			}
			if rr.Code != http.StatusOK {
				t.Errorf("status: want 200, got %d", rr.Code)
			}
		})
	}
}

// -------------------------------------------------------------------
// Route matching: known route passes; unknown route handled
// per-mode
// -------------------------------------------------------------------

func TestOpenAPIValidate_KnownRoute_ReachesHandler(t *testing.T) {
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:              specPathForTests(t),
		RejectInvalidRequests: true,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}

	handlerCalled := false
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"meta":"ok"}`))
	}))

	// GET /api/meta is in the spec; no query params required.
	req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Fatal("handler not reached for documented /api/meta")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rr.Code)
	}
}

func TestOpenAPIValidate_UnknownRoute(t *testing.T) {
	cases := []struct {
		name           string
		strictReq      bool
		wantHandlerHit bool
		wantStatus     int
	}{
		{"strict mode rejects with 404", true, false, http.StatusNotFound},
		{"permissive mode passes through", false, true, http.StatusOK},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingLogf{}
			v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
				SpecPath:              specPathForTests(t),
				RejectInvalidRequests: tc.strictReq,
				Logf:                  rec.f,
			})
			if err != nil {
				t.Fatalf("validator init: %v", err)
			}
			handlerCalled := false
			h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			}))
			req := httptest.NewRequest(http.MethodGet, "/api/this-is-not-a-real-endpoint", nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if handlerCalled != tc.wantHandlerHit {
				t.Errorf("handler called: want %v, got %v", tc.wantHandlerHit, handlerCalled)
			}
			if rr.Code != tc.wantStatus {
				t.Errorf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.strictReq {
				// Verify the unified ErrorResponse envelope.
				var body ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
				}
				if body.Status != http.StatusNotFound {
					t.Errorf("body.status: want 404, got %d", body.Status)
				}
				if !strings.Contains(body.Error, "OpenAPI") {
					t.Errorf("body.error should mention OpenAPI, got %q", body.Error)
				}
			}
			lines := rec.snapshot()
			foundLog := false
			for _, ln := range lines {
				if strings.Contains(ln, "route miss") {
					foundLog = true
					break
				}
			}
			if !foundLog {
				t.Errorf("expected `route miss` log line, got %v", lines)
			}
		})
	}
}

// -------------------------------------------------------------------
// Request body validation
// -------------------------------------------------------------------

// TestOpenAPIValidate_InvalidRequestBody — POST /api/feedback
// requires `symptom` per the spec. A request missing it must be
// rejected in strict mode and merely logged in permissive mode.
func TestOpenAPIValidate_InvalidRequestBody(t *testing.T) {
	cases := []struct {
		name        string
		strictReq   bool
		body        string
		wantHandler bool
		wantStatus  int
	}{
		{
			name:        "valid body passes through",
			strictReq:   true,
			body:        `{"symptom":"freeze on import"}`,
			wantHandler: true,
			wantStatus:  http.StatusOK,
		},
		{
			name:        "missing required field rejected in strict mode",
			strictReq:   true,
			body:        `{}`,
			wantHandler: false,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing required field passes in permissive mode",
			strictReq:   false,
			body:        `{}`,
			wantHandler: true,
			wantStatus:  http.StatusOK,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingLogf{}
			v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
				SpecPath:              specPathForTests(t),
				RejectInvalidRequests: tc.strictReq,
				Logf:                  rec.f,
			})
			if err != nil {
				t.Fatalf("validator init: %v", err)
			}
			handlerCalled := false
			h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalled = true
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))

			req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)

			if handlerCalled != tc.wantHandler {
				t.Errorf("handler called: want %v, got %v (status=%d body=%q)",
					tc.wantHandler, handlerCalled, rr.Code, rr.Body.String())
			}
			if rr.Code != tc.wantStatus {
				t.Errorf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if !tc.wantHandler && tc.strictReq {
				var body ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
				}
				if body.Status != http.StatusBadRequest {
					t.Errorf("body.status: want 400, got %d", body.Status)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// Response validation (strict)
// -------------------------------------------------------------------

// TestOpenAPIValidate_ResponseValidation_PassesThroughConformantBody —
// when the handler emits a valid response and strict response
// validation is enabled, the body must reach the client unchanged.
func TestOpenAPIValidate_ResponseValidation_PassesThroughConformantBody(t *testing.T) {
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:               specPathForTests(t),
		RejectInvalidResponses: true,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}
	body := `{"card_name":"Sol Ring","win_rate":0.55,"games_played":100,"wins":55,"losses":45,"inclusion_rate":0.4,"decks_using":10,"total_decks":25,"top_commanders":[],"bracket_distribution":[]}`
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/card-stats/card/Sol%20Ring", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (body=%q)", rr.Code, rr.Body.String())
	}
	if got := rr.Body.String(); got != body {
		t.Errorf("body mutated by middleware: want %q, got %q", body, got)
	}
}

// TestOpenAPIValidate_ResponseValidation_RejectsBadStatus — the
// CardStatsOverview endpoint documents 200 + 400 only. A handler
// emitting 418 must be substituted with a 500 ErrorResponse in
// strict response mode.
func TestOpenAPIValidate_ResponseValidation_RejectsBadStatus(t *testing.T) {
	rec := &recordingLogf{}
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:               specPathForTests(t),
		RejectInvalidResponses: true,
		Logf:                   rec.f,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(`{"surprise":"teapot"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/card-stats/card/Sol%20Ring", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status: want 500 (validator substitution), got %d (body=%q)", rr.Code, rr.Body.String())
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
	}
	if body.Status != http.StatusInternalServerError {
		t.Errorf("body.status: want 500, got %d", body.Status)
	}
	if !strings.Contains(strings.ToLower(body.Error), "openapi") {
		t.Errorf("body.error: want OpenAPI mention, got %q", body.Error)
	}
	lines := rec.snapshot()
	foundLog := false
	for _, ln := range lines {
		if strings.Contains(ln, "invalid response") {
			foundLog = true
			break
		}
	}
	if !foundLog {
		t.Errorf("expected `invalid response` log line, got %v", lines)
	}
}

// TestOpenAPIValidate_ResponseValidation_LogOnly — when response
// validation is OFF, a misbehaving handler's body reaches the
// client even though the validator may notice the divergence.
func TestOpenAPIValidate_ResponseValidation_LogOnly(t *testing.T) {
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath: specPathForTests(t),
		// Both reject flags false — the validator runs in shadow
		// mode and never mutates the response.
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}
	rawBody := `{"surprise":"teapot"}`
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte(rawBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/card-stats/card/Sol%20Ring", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("status: want 418 (handler unchanged), got %d", rr.Code)
	}
	if got := rr.Body.String(); got != rawBody {
		t.Errorf("body should not be mutated in log-only mode: want %q, got %q", rawBody, got)
	}
}

// -------------------------------------------------------------------
// Strict + buffered response: handler that streams must still
// arrive at the client intact when the body conforms.
// -------------------------------------------------------------------

func TestOpenAPIValidate_BufferedWriter_PreservesHeaderOrder(t *testing.T) {
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:               specPathForTests(t),
		RejectInvalidResponses: true,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}

	// Use /api/meta which the spec documents with EmptyOk
	// (additionalProperties: true) — any JSON body passes schema.
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "preserved")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"engine":"hexdek"}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/meta", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rr.Code)
	}
	if rr.Header().Get("X-Custom") != "preserved" {
		t.Errorf("X-Custom header dropped by buffered writer: got %q", rr.Header().Get("X-Custom"))
	}
	if got := rr.Body.String(); !strings.Contains(got, "hexdek") {
		t.Errorf("body should pass through verbatim: got %q", got)
	}
}

// -------------------------------------------------------------------
// Helper: confirm summariseError truncates correctly so an
// ErrorResponse from the middleware never balloons.
// -------------------------------------------------------------------

func TestSummariseError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want string
	}{
		{"nil", nil, ""},
		{"single line", errFromString("boom"), "boom"},
		{"multi line keeps only first", errFromString("line1\nline2\nline3"), "line1"},
		{
			name: "long messages get truncated",
			in:   errFromString(strings.Repeat("x", 500)),
			want: strings.Repeat("x", 240) + "...",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := summariseError(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// errFromString is a tiny adapter to build an error from a string
// without dragging in errors.New everywhere.
type stringErr string

func (s stringErr) Error() string  { return string(s) }
func errFromString(s string) error { return stringErr(s) }

// -------------------------------------------------------------------
// Defensive: ensure the bufferedResponseWriter actually buffers and
// doesn't leak partial writes to the real client when validation
// rejects.
// -------------------------------------------------------------------

func TestOpenAPIValidate_BufferedWriter_RejectionDoesNotLeakBody(t *testing.T) {
	v, err := NewOpenAPIValidator(OpenAPIValidatorOpts{
		SpecPath:               specPathForTests(t),
		RejectInvalidResponses: true,
	})
	if err != nil {
		t.Fatalf("validator init: %v", err)
	}
	rawBody := `{"surprise":"teapot"}`
	h := v.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTeapot) // not in spec
		_, _ = w.Write([]byte(rawBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/card-stats/card/Sol%20Ring", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// Body must not contain the handler's raw payload — only the
	// validator's substitution ErrorResponse.
	if bytes.Contains(rr.Body.Bytes(), []byte("surprise")) {
		t.Errorf("rejected response leaked handler body to client: %q", rr.Body.String())
	}
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rr.Code)
	}
}
