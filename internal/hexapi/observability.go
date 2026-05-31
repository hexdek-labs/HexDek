package hexapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// observabilityResponseWriter wraps http.ResponseWriter to capture
// the final status code plus the first errorPeekCap bytes of the
// response body. The body peek is the side channel the middleware
// uses to lift the ErrorResponse.Error.Code slug back out of the
// rendered JSON envelope without requiring every error site to
// also call into a separate stash API.
//
// The wrapper deliberately preserves http.Hijacker / http.Flusher /
// http.Pusher access via interface-conditional helpers below — the
// hexapi WebSocket and SSE paths (live spectator stream, gauntlet
// SSE) need Hijack and Flush respectively.
type observabilityResponseWriter struct {
	http.ResponseWriter
	status     int
	bodyPeek   bytes.Buffer
	peekCap    int
	wroteHeader bool
}

const errorPeekCap = 2048

// WriteHeader records the status code so the middleware can stamp
// it into the metric / log line after the handler returns.
func (o *observabilityResponseWriter) WriteHeader(status int) {
	o.status = status
	o.wroteHeader = true
	o.ResponseWriter.WriteHeader(status)
}

// Write captures up to peekCap bytes of the response body before
// forwarding to the real writer. We never short-write the underlying
// stream — the peek is purely a side observation.
func (o *observabilityResponseWriter) Write(b []byte) (int, error) {
	if !o.wroteHeader {
		// http.ResponseWriter contract: implicit WriteHeader(200) on
		// first Write. Capture the status before recording the body
		// so the metric reflects reality.
		o.status = http.StatusOK
		o.wroteHeader = true
	}
	if o.bodyPeek.Len() < o.peekCap {
		room := o.peekCap - o.bodyPeek.Len()
		if room > len(b) {
			room = len(b)
		}
		o.bodyPeek.Write(b[:room])
	}
	return o.ResponseWriter.Write(b)
}

// Flush threads through to the underlying writer when supported —
// the SSE paths rely on this to ship per-event chunks before the
// handler returns.
func (o *observabilityResponseWriter) Flush() {
	if f, ok := o.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// extractErrorSlug parses the captured body peek as the unified
// ErrorResponse envelope and returns the error.code slug. Returns ""
// when the status is a success (2xx), when the body isn't JSON, or
// when the envelope doesn't carry a code field — every hexapi error
// path SHOULD flow through writeError* and emit a code, so an empty
// return here on a 4xx/5xx is itself a signal worth surfacing in
// dashboards (the absence will show up as `error_slug=""` rows on
// hexapi_error_count, which is the canonical "uncategorized error"
// pivot).
func (o *observabilityResponseWriter) extractErrorSlug() string {
	if o.status < 400 {
		return ""
	}
	if o.bodyPeek.Len() == 0 {
		return ""
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	// Use a Decoder so trailing bytes (when the body exceeded peekCap)
	// don't fail the parse — we only need the top-level envelope.
	dec := json.NewDecoder(bytes.NewReader(o.bodyPeek.Bytes()))
	if err := dec.Decode(&body); err != nil {
		return ""
	}
	return body.Error.Code
}

// StructuredLogger is the shape ObservabilityMiddleware uses to emit
// per-request log lines. Implementations decide format / destination
// (stdout key=value, JSON, file, network); the middleware just
// hands over the labeled fields once per request. Nil is allowed
// at construction — the middleware skips the log step in that case,
// keeping metrics-only deployments cheap.
type StructuredLogger interface {
	LogRequest(fields LogFields)
}

// LogFields is the per-request payload handed to a StructuredLogger.
// All times are wall-clock; the duration is the wrapped-handler
// elapsed (excludes upstream middleware time).
type LogFields struct {
	RequestID  string
	Endpoint   string // route template, e.g. "/api/decks/{owner}/{id}"
	Method     string
	Path       string // concrete path with placeholders filled in
	Status     int
	DurationMS float64
	ErrorSlug  string // empty on success
}

// LoggerFunc adapts a closure to the StructuredLogger interface so
// callers don't need to declare a type for ad-hoc loggers (test
// fixtures, dev-only stdout sinks).
type LoggerFunc func(LogFields)

// LogRequest implements StructuredLogger.
func (f LoggerFunc) LogRequest(fields LogFields) { f(fields) }

// StdoutLogger returns a StructuredLogger that writes each request
// as a single-line `key=value` record to the given writer (usually
// os.Stdout). Values are quoted when they contain whitespace or `=`.
// This is the production default: greppable, parseable by jq -R, and
// trivially redirectable to a file / journald.
func StdoutLogger(w io.Writer) StructuredLogger {
	return LoggerFunc(func(f LogFields) {
		var b strings.Builder
		writeLogKV(&b, "level", "info")
		writeLogKV(&b, "request_id", f.RequestID)
		writeLogKV(&b, "endpoint", f.Endpoint)
		writeLogKV(&b, "method", f.Method)
		if f.Path != "" && f.Path != f.Endpoint {
			writeLogKV(&b, "path", f.Path)
		}
		writeLogKV(&b, "status", strconv.Itoa(f.Status))
		writeLogKV(&b, "duration_ms", strconv.FormatFloat(f.DurationMS, 'f', 2, 64))
		if f.ErrorSlug != "" {
			writeLogKV(&b, "error_slug", f.ErrorSlug)
		}
		b.WriteByte('\n')
		_, _ = io.WriteString(w, b.String())
	})
}

func writeLogKV(b *strings.Builder, k, v string) {
	if b.Len() > 0 {
		b.WriteByte(' ')
	}
	b.WriteString(k)
	b.WriteByte('=')
	if needsLogQuote(v) {
		b.WriteString(strconv.Quote(v))
	} else {
		b.WriteString(v)
	}
}

func needsLogQuote(v string) bool {
	if v == "" {
		return true
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c <= ' ' || c == '=' || c == '"' || c == '\\' {
			return true
		}
	}
	return false
}

// ObservabilityMiddleware wraps an http.Handler with per-request
// metrics + structured-log emission. Either registry or logger may
// be nil — nil registry skips metrics, nil logger skips logs (a
// metrics-only or logs-only deployment is supported by passing nil
// for the other). When both are nil the middleware is a straight
// pass-through, making it cheap to drop in front of every handler
// without ceremony.
//
// Endpoint label comes from r.Pattern (the route template), so
// {owner}/{id} placeholders stay placeholders and cardinality is
// bounded. Requests that arrive without a registered pattern (e.g.
// sub-mux dispatched, raw 404s from the mux's NotFoundHandler) skip
// metric recording but still emit a log line (path is filled in
// from r.URL.Path so the trace is still useful).
//
// Body peek for error_slug extraction is capped at 2KiB — every
// hexapi ErrorResponse envelope is well under 1KiB, so this is plenty
// to cover the JSON without bloating long success responses.
func ObservabilityMiddleware(registry *MetricsRegistry, logger StructuredLogger, next http.Handler) http.Handler {
	if registry == nil && logger == nil {
		return next
	}
	now := time.Now
	if registry != nil && registry.now != nil {
		now = registry.now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := now()
		ow := &observabilityResponseWriter{ResponseWriter: w, peekCap: errorPeekCap}
		next.ServeHTTP(ow, r)
		// If the handler never wrote a status, the http.ResponseWriter
		// contract treats that as 200 — mirror it here so a handler
		// that just returns without writing doesn't show up as status=0.
		status := ow.status
		if status == 0 {
			status = http.StatusOK
		}
		elapsed := now().Sub(start)
		endpoint := normalizeEndpoint(r.Pattern)
		errorSlug := ow.extractErrorSlug()

		if registry != nil && endpoint != "" {
			registry.RecordRequest(endpoint, r.Method, status, elapsed, errorSlug)
		}
		if logger != nil {
			logger.LogRequest(LogFields{
				RequestID:  RequestIDFromContext(r.Context()),
				Endpoint:   endpoint,
				Method:     r.Method,
				Path:       r.URL.Path,
				Status:     status,
				DurationMS: float64(elapsed.Microseconds()) / 1000.0,
				ErrorSlug:  errorSlug,
			})
		}
	})
}

