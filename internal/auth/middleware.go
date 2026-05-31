package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type contextKey struct{ name string }

// ctxKeySession is the key under which an authenticated *Session is stored
// on the request context.
var ctxKeySession = contextKey{name: "hexdek/auth.session"}

// FromContext returns the authenticated session for a request, or nil if
// the request was not authenticated.
func FromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(ctxKeySession).(*Session)
	return s
}

// withSession attaches a session to a context.
func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, ctxKeySession, s)
}

// extractToken pulls a session token from the request, in order of preference:
//  1. Authorization: Bearer <token> header
//  2. ?token=<token> query param (used by WebSocket clients that can't set headers)
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(strings.ToLower(h), "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	return r.URL.Query().Get("token")
}

// authErrorCodeMessage maps a ValidateSession / ValidateAPIKey error to
// a safe client-facing pair of (stable machine-readable slug, user-
// facing message). The slug feeds the unified envelope's `error.code`
// field so clients can switch on it without parsing the message;
// sentinel errors (invalid / expired token) tell the client exactly
// what's wrong so it can re-prompt for login. Unexpected errors (DB
// faults, driver messages) are logged server-side and returned as a
// generic "unauthorized" so a probing attacker can't enumerate the
// persistence layer.
// Slug taxonomy:
//   - invalid_token   — session bearer token not found in DB
//   - session_expired — session row exists but ExpiresAt is past
//   - invalid_api_key — any API-key failure (collapsed to one slug per the
//     same enumeration-leak guard that collapses the message)
//   - unauthorized    — unexpected error (logged server-side, generic wire)
func authErrorCodeMessage(err error) (code, message string) {
	switch {
	case errors.Is(err, ErrInvalidToken):
		return "invalid_token", "unauthorized: invalid or unknown session token"
	case errors.Is(err, ErrSessionExpired):
		return "session_expired", "unauthorized: session token expired"
	// API-key errors collapse to a single user-visible message so
	// the wire response can't distinguish "key doesn't exist" from
	// "key revoked" from "key expired" — server-side log keeps the
	// granular reason for audit.
	case errors.Is(err, ErrInvalidAPIKey),
		errors.Is(err, ErrAPIKeyExpired),
		errors.Is(err, ErrAPIKeyRevoked):
		return "invalid_api_key", "unauthorized: invalid api key"
	default:
		log.Printf("auth: validate credential: %v", err)
		return "unauthorized", "unauthorized"
	}
}

// errorBody mirrors hexapi.ErrorBody as the nested error envelope. Kept
// duplicated rather than imported because internal/auth cannot import
// internal/hexapi (hexapi already depends on auth; the reverse import
// would create a cycle). Wire shape MUST stay bit-identical to
// hexapi.ErrorBody so every 401 from this middleware decodes the same
// way as the 309 other unified-envelope sites established by PR #778.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// errorResponse mirrors hexapi.ErrorResponse — see errorBody for the
// rationale behind duplicating instead of importing.
type errorResponse struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
	Status    int       `json:"status"`
}

// requestIDHeader mirrors hexapi.RequestIDHeader. Duplicated for the
// same import-cycle reason as errorBody. If RequestIDMiddleware ran
// before us it will have stamped the header on the response writer;
// if not, the field stays empty and `omitempty` drops it from the wire.
const requestIDHeader = "X-Request-Id"

// writeAuthError writes a 401 in the same nested-envelope shape every
// other hexapi handler uses (PR #778 — `writeError`). Replaces the
// previous plain-text `http.Error` that broke the unified contract for
// every protected endpoint's auth-failure path.
func writeAuthError(w http.ResponseWriter, err error) {
	code, message := authErrorCodeMessage(err)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	reqID := w.Header().Get(requestIDHeader)
	w.WriteHeader(http.StatusUnauthorized)
	body := errorResponse{
		Error:     errorBody{Code: code, Message: message},
		RequestID: reqID,
		Status:    http.StatusUnauthorized,
	}
	if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
		log.Printf("auth writeAuthError encode: %v", encErr)
	}
}

// validateCredential dispatches a bearer token to either the API-key
// validator (when the token has the hxk_ shape) or the session
// validator (everything else). Returns a *Session synthesized from
// whichever credential validated — downstream FromContext callers
// keep working without knowing which credential type was used.
//
// API-key path synthesizes a Session with:
//   - Token = "apikey:<id>"  so server-side audit can tell credential
//                            types apart without exposing the secret
//   - DeviceID = key.DeviceID (the identity, which IS what downstream
//                              code wants)
//   - CreatedAt / ExpiresAt / LastUsedAt = from the api_key row
//
// This preserves the existing FromContext(ctx) *Session contract so
// every existing protected handler keeps working unchanged.
func validateCredential(ctx context.Context, database *sql.DB, token string) (*Session, error) {
	if IsAPIKeyShape(token) {
		k, err := ValidateAPIKey(ctx, database, token)
		if err != nil {
			return nil, err
		}
		return &Session{
			Token:      "apikey:" + k.ID,
			DeviceID:   k.DeviceID,
			CreatedAt:  k.CreatedAt,
			ExpiresAt:  k.ExpiresAt,
			LastUsedAt: k.LastUsedAt,
		}, nil
	}
	return ValidateSession(ctx, database, token)
}

// Required wraps a handler to require either a valid session token
// or a valid API key. Unauthenticated requests get a 401 response.
// The credential type is opaque to the wrapped handler — both
// surface as a *Session via FromContext.
func Required(database *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		s, err := validateCredential(r.Context(), database, token)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), s)))
	})
}

// RequiredFunc is the http.HandlerFunc-friendly variant of Required.
func RequiredFunc(database *sql.DB, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		s, err := validateCredential(r.Context(), database, token)
		if err != nil {
			writeAuthError(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), s)))
	}
}
