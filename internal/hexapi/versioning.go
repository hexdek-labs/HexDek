package hexapi

import (
	"net/http"
	"strings"
	"time"
)

// VersioningOpts configures VersioningMiddleware. Both fields are
// optional — a zero-value struct selects sensible defaults
// (defaultSunsetAt for the Sunset header, defaultSuccessorPath for
// the legacy → v1 path rewrite).
type VersioningOpts struct {
	// SunsetAt is the date stamped into the Sunset response header on
	// legacy /api/* responses. Zero means "use defaultSunsetAt".
	SunsetAt time.Time

	// SuccessorPath rewrites a legacy /api/* path into its /api/v1/*
	// equivalent for the Link header. Nil means "use defaultSuccessorPath".
	SuccessorPath func(legacy string) string
}

// defaultSunsetAt is the package-default deprecation horizon for
// legacy /api/* endpoints. Picked far enough in the future to give
// clients time to migrate, but stable enough that the tests can
// pin its presence without pinning its exact value.
var defaultSunsetAt = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

// VersioningMiddleware is the REST API versioning shim. Three branches:
//   - /api/v1/*  → strip "/v1" before the next handler sees the
//     request, stamp X-HexDek-API-Version: v1 on the response.
//   - /api/*     → reach the next handler unchanged, attach
//     Deprecation: true + Sunset + Link successor-version +
//     X-HexDek-API-Version: legacy so clients can detect the
//     migration coming.
//   - everything else (/ws/, /graphql, /share/, /, ...) → pass
//     through untouched. The middleware never pollutes non-REST
//     endpoints with version headers.
func VersioningMiddleware(next http.Handler, opts VersioningOpts) http.Handler {
	sunset := opts.SunsetAt
	if sunset.IsZero() {
		sunset = defaultSunsetAt
	}
	successor := opts.SuccessorPath
	if successor == nil {
		successor = defaultSuccessorPath
	}
	sunsetHeader := sunset.Format(http.TimeFormat)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/api/v1/") || path == "/api/v1":
			// Strip the "/v1" segment: /api/v1/decks/123 → /api/decks/123.
			// Clone the URL so we don't mutate the caller's request
			// object — handlers downstream may stash references to the
			// original request.
			r2 := *r
			u2 := *r.URL
			u2.Path = "/api" + strings.TrimPrefix(path, "/api/v1")
			r2.URL = &u2
			w.Header().Set("X-HexDek-API-Version", "v1")
			next.ServeHTTP(w, &r2)
		case strings.HasPrefix(path, "/api/") || path == "/api":
			w.Header().Set("X-HexDek-API-Version", "legacy")
			w.Header().Set("Deprecation", "true")
			w.Header().Set("Sunset", sunsetHeader)
			w.Header().Set("Link", "<"+successor(path)+`>;rel="successor-version"`)
			next.ServeHTTP(w, r)
		default:
			next.ServeHTTP(w, r)
		}
	})
}

// defaultSuccessorPath rewrites a legacy /api/* path into its v1
// equivalent. Defensive: an already-versioned /api/v1/* path returns
// unchanged (guards against a caller accidentally feeding the
// already-rewritten path back through); a non-/api/ path returns
// unchanged (the middleware never invokes this on non-/api/ paths,
// but the function is defensively callable).
func defaultSuccessorPath(legacy string) string {
	if !strings.HasPrefix(legacy, "/api/") && legacy != "/api" {
		return legacy
	}
	if strings.HasPrefix(legacy, "/api/v1/") || legacy == "/api/v1" {
		return legacy
	}
	return "/api/v1" + strings.TrimPrefix(legacy, "/api")
}
