package auth

import (
	"context"
	"database/sql"
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

// authErrorMessage maps a ValidateSession error to a safe client-facing 401
// body. Sentinel errors (invalid / expired token) tell the client exactly
// what's wrong so it can re-prompt for login. Unexpected errors (DB faults,
// driver messages) are logged server-side and returned as a generic
// "unauthorized" so a probing attacker can't enumerate the persistence
// layer.
func authErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrInvalidToken):
		return "unauthorized: invalid or unknown session token"
	case errors.Is(err, ErrSessionExpired):
		return "unauthorized: session token expired"
	// API-key errors collapse to a single user-visible message so
	// the wire response can't distinguish "key doesn't exist" from
	// "key revoked" from "key expired" — server-side log keeps the
	// granular reason for audit.
	case errors.Is(err, ErrInvalidAPIKey),
		errors.Is(err, ErrAPIKeyExpired),
		errors.Is(err, ErrAPIKeyRevoked):
		return "unauthorized: invalid api key"
	default:
		log.Printf("auth: validate credential: %v", err)
		return "unauthorized"
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
			http.Error(w, authErrorMessage(err), http.StatusUnauthorized)
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
			http.Error(w, authErrorMessage(err), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), s)))
	}
}
