package hexapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// OpenAPIValidatorOpts configures NewOpenAPIValidator. The zero value
// is valid for read-only spec loading; populate SpecPath at minimum
// before calling NewOpenAPIValidator.
type OpenAPIValidatorOpts struct {
	// SpecPath is the path to the OpenAPI 3.x YAML on disk.
	// `docs/hexapi-openapi.yaml` in this repo.
	SpecPath string

	// RejectInvalidRequests, when true, returns a 4xx ErrorResponse
	// for requests that the validator finds non-conformant —
	// undocumented route (404), wrong method (404 via route miss),
	// missing required params, mismatched request body schema, etc.
	// When false, the validator logs the violation and lets the
	// request proceed to the handler unchanged. Default false; the
	// production wiring in cmd/hexdek-server should stay permissive
	// until the spec has been hardened in shadow mode.
	RejectInvalidRequests bool

	// RejectInvalidResponses, when true, replaces the wrapped
	// handler's response with a 500 ErrorResponse whenever the
	// response body, status, or headers diverge from the spec. This
	// option buffers the entire response body in memory before
	// flushing it to the client — never enable for large-payload or
	// streaming endpoints in production. Implied false unless
	// explicitly set.
	RejectInvalidResponses bool

	// SkipPaths is the list of path prefixes the validator should
	// never touch — WebSocket upgrades, Server-Sent Event streams,
	// the contributor compute socket, and any other route whose
	// content type OpenAPI cannot meaningfully describe. Compared
	// with strings.HasPrefix; the default list is constructed by
	// defaultSkipPaths when SkipPaths is nil.
	SkipPaths []string

	// Logf is invoked once per validation violation with a short
	// human-readable description (method, path, error). Defaults
	// to log.Printf when nil.
	Logf func(format string, args ...any)
}

// OpenAPIValidator is the middleware-producing handle around the
// loaded OpenAPI document. Construct via NewOpenAPIValidator; wrap a
// handler with Middleware to enforce the spec at request boundary.
type OpenAPIValidator struct {
	doc    *openapi3.T
	router routers.Router
	opts   OpenAPIValidatorOpts
}

// NewOpenAPIValidator loads the spec at opts.SpecPath, validates the
// spec itself, and returns a validator ready for Middleware wiring.
// Returns an error if the file is missing or the spec is malformed —
// callers should fail-fast on this rather than silently ship an
// inert middleware.
func NewOpenAPIValidator(opts OpenAPIValidatorOpts) (*OpenAPIValidator, error) {
	if opts.SpecPath == "" {
		return nil, fmt.Errorf("openapi validator: SpecPath required")
	}
	if opts.Logf == nil {
		opts.Logf = log.Printf
	}
	if opts.SkipPaths == nil {
		opts.SkipPaths = defaultSkipPaths()
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(opts.SpecPath)
	if err != nil {
		return nil, fmt.Errorf("openapi validator: load %s: %w", opts.SpecPath, err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		return nil, fmt.Errorf("openapi validator: spec invalid: %w", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("openapi validator: build router: %w", err)
	}
	return &OpenAPIValidator{doc: doc, router: router, opts: opts}, nil
}

// defaultSkipPaths returns the prefix list the validator skips when
// no caller-supplied SkipPaths is configured. WebSockets and SSE
// streams cannot be modelled by OpenAPI body schemas, and the
// Heimdall PDF endpoint serves a binary the validator would refuse.
func defaultSkipPaths() []string {
	return []string{
		"/ws/",
		"/api/contrib/connect",
		"/api/decks/", // any deck-scoped SSE under the same prefix
		"/api/tournaments/",
		"/api/games/",                              // /games/{id}/summary.pdf etc.
		"/decks/", "/cards/", "/operator/",         // HTML share pages
		"/spectate", "/leaderboard",                // top-level share pages
		"/api/card-art/",                           // binary art
	}
}

// shouldSkip returns true when the validator must bypass r without
// any validation work — e.g. for WS / SSE / binary endpoints.
func (v *OpenAPIValidator) shouldSkip(path string) bool {
	for _, p := range v.opts.SkipPaths {
		if p == path || strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// Middleware returns an http.Handler that wraps next with request /
// optional response validation against the loaded spec. Behaviour is
// governed by OpenAPIValidatorOpts:
//
//   - Requests are always run through the validator (modulo
//     SkipPaths). Failures are logged via opts.Logf.
//   - When RejectInvalidRequests is true, validation failures stop
//     the chain and emit an ErrorResponse via writeError.
//   - When RejectInvalidResponses is true, the response body is
//     buffered, validated, and either flushed to the client or
//     replaced with a 500 ErrorResponse. Memory-intensive — leave
//     off in production.
func (v *OpenAPIValidator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v.shouldSkip(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		ctx := r.Context()
		route, pathParams, err := v.router.FindRoute(r)
		if err != nil {
			v.opts.Logf("openapi: route miss %s %s: %v", r.Method, r.URL.Path, err)
			if v.opts.RejectInvalidRequests {
				writeError(w, http.StatusNotFound, "route not in OpenAPI spec")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		reqInput := &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
		}
		if err := openapi3filter.ValidateRequest(ctx, reqInput); err != nil {
			v.opts.Logf("openapi: invalid request %s %s: %v", r.Method, r.URL.Path, err)
			if v.opts.RejectInvalidRequests {
				writeError(w, http.StatusBadRequest, "request does not match OpenAPI spec: "+summariseError(err))
				return
			}
		}

		if !v.opts.RejectInvalidResponses {
			next.ServeHTTP(w, r)
			return
		}

		// Strict response validation: buffer everything the handler
		// writes, validate, then flush or replace. IncludeResponseStatus
		// flips the default permissive behaviour — undocumented status
		// codes (the handler returned 418 for an endpoint that only
		// documents 200 + 400) become errors instead of pass-throughs.
		bw := &bufferedResponseWriter{w: w, status: http.StatusOK}
		next.ServeHTTP(bw, r)
		respOptions := &openapi3filter.Options{IncludeResponseStatus: true}
		respInput := &openapi3filter.ResponseValidationInput{
			RequestValidationInput: reqInput,
			Status:                 bw.status,
			Header:                 bw.Header(),
			Body:                   io.NopCloser(bytes.NewReader(bw.body.Bytes())),
			Options:                respOptions,
		}
		if err := openapi3filter.ValidateResponse(ctx, respInput); err != nil {
			v.opts.Logf("openapi: invalid response %s %s status=%d: %v",
				r.Method, r.URL.Path, bw.status, err)
			writeError(w, http.StatusInternalServerError,
				"response does not match OpenAPI spec: "+summariseError(err))
			return
		}
		// Conforming response — flush captured bytes to the client.
		bw.flush()
	})
}

// summariseError flattens kin-openapi's nested error type into a
// single short string suitable for an ErrorResponse body. The full
// detail still goes to the log; clients get the high-water-mark
// reason without leaking schema internals.
func summariseError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// kin-openapi error messages can be multi-line YAML-ish dumps —
	// keep just the first line so the JSON envelope stays small.
	if idx := strings.IndexByte(msg, '\n'); idx > 0 {
		msg = msg[:idx]
	}
	if len(msg) > 240 {
		msg = msg[:240] + "..."
	}
	return msg
}

// bufferedResponseWriter is the in-memory ResponseWriter the strict
// response-validation path wraps around. It defers WriteHeader to
// flush(), so an invalid-response substitution can replace status +
// body cleanly. http.Flusher passthrough is intentionally NOT
// provided — strict response validation is incompatible with SSE,
// which is why SSE endpoints belong in SkipPaths.
type bufferedResponseWriter struct {
	w       http.ResponseWriter
	status  int
	headers http.Header
	body    bytes.Buffer
	wrote   bool
}

func (bw *bufferedResponseWriter) Header() http.Header {
	if bw.headers == nil {
		// Lazily snapshot the underlying writer's header map so the
		// handler can mutate it but our flush controls when those
		// mutations land on the wire.
		bw.headers = bw.w.Header()
	}
	return bw.headers
}

func (bw *bufferedResponseWriter) WriteHeader(status int) {
	if !bw.wrote {
		bw.status = status
		bw.wrote = true
	}
}

func (bw *bufferedResponseWriter) Write(b []byte) (int, error) {
	if !bw.wrote {
		bw.WriteHeader(http.StatusOK)
	}
	return bw.body.Write(b)
}

// flush writes the buffered status + body to the underlying
// ResponseWriter. Called only when validation passes; the
// reject-path uses writeError directly instead.
func (bw *bufferedResponseWriter) flush() {
	bw.w.WriteHeader(bw.status)
	_, _ = bw.w.Write(bw.body.Bytes())
}

// ValidateLoadedSpec is a convenience for tests / startup checks:
// load the spec at path and confirm it parses + self-validates. The
// returned doc can be ignored — callers usually only care about the
// error.
func ValidateLoadedSpec(path string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := doc.Validate(context.Background()); err != nil {
		return doc, err
	}
	return doc, nil
}
