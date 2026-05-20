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
	default:
		log.Printf("auth: validate session: %v", err)
		return "unauthorized"
	}
}

// Required wraps a handler to require a valid session token. Unauthenticated
// requests get a 401 response.
func Required(database *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		s, err := ValidateSession(r.Context(), database, token)
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
		s, err := ValidateSession(r.Context(), database, token)
		if err != nil {
			http.Error(w, authErrorMessage(err), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(withSession(r.Context(), s)))
	}
}
