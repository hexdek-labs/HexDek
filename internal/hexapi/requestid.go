package hexapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestIDHeader is the canonical header name for the per-request
// trace ID. We accept the same header on inbound requests (so callers
// that already generate their own IDs — load balancers, the React
// frontend's fetch wrapper — get their value preserved through to
// logs and error bodies) and always echo it on the response.
const RequestIDHeader = "X-Request-Id"

type requestIDCtxKey struct{}

// RequestIDFromContext returns the request ID attached to ctx by
// RequestIDMiddleware, or the empty string when no middleware has
// run. Handlers that want to log with the trace ID should use this
// rather than re-reading the header (the request header is the
// CALLER's input; the context value is the post-middleware authority
// that also lives on the response).
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// RequestIDMiddleware ensures every request entering the hexapi layer
// has an X-Request-Id that round-trips end-to-end:
//
//  1. If the inbound request already carries X-Request-Id, that value
//     is preserved (load balancers, the frontend fetch wrapper, and
//     test harnesses can pin a known ID).
//  2. Otherwise a fresh 16-byte hex ID is generated.
//  3. The value is set on the response header BEFORE the wrapped
//     handler runs, so writeError* helpers can read it back via
//     w.Header().Get(RequestIDHeader) when shaping the error envelope
//     even after the handler has been chained through other wrappers.
//  4. The value is also stashed on the request context for handlers
//     that want to include it in structured logs.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" || !isPlausibleRequestID(id) {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDCtxKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newRequestID returns a 32-char lowercase hex string from
// crypto/rand. We avoid the uuid package to keep this file dependency-
// free; the value only needs to be unique-per-request and 128 bits of
// entropy is more than enough.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on a healthy host is essentially
		// impossible — but if it ever does happen we don't want to
		// fail the whole request, just return a placeholder so the
		// envelope stays well-formed.
		return "norand"
	}
	return hex.EncodeToString(b[:])
}

// isPlausibleRequestID guards against caller-supplied junk getting
// echoed into our logs. We accept anything 8-128 chars made of
// printable ASCII excluding whitespace and control characters.
// Anything longer or weirder gets replaced with a fresh ID.
func isPlausibleRequestID(s string) bool {
	if len(s) < 8 || len(s) > 128 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x21 || c > 0x7e {
			return false
		}
	}
	return true
}
