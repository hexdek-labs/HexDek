package hexapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ErrorBody is the nested error envelope every hexapi handler returns
// when it signals a non-success HTTP status. Code is a stable
// machine-readable slug (e.g. "bad_request", "not_found",
// "insufficient_credits") that clients can switch on without parsing
// the human-readable Message. Details carries optional structured
// context (e.g. {"balance": 7, "needed": 5} for the gauntlet credits
// gate) and is omitted from the wire when nil.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorResponse is the single shape every hexapi handler returns when
// it signals a non-success HTTP status. The wire contract is:
//
//	{
//	  "error":      {"code": "bad_request", "message": "...", "details": {...}},
//	  "request_id": "<uuid>",
//	  "status":     400
//	}
//
// `status` mirrors the HTTP status code so it survives intermediaries
// that strip the header. `request_id` echoes the X-Request-Id header
// that RequestIDMiddleware installed on the response (and on the
// request if the caller did not supply one), making errors traceable
// end-to-end through logs and bug reports.
type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
	Status    int       `json:"status"`
}

// writeError writes a JSON ErrorResponse with the given HTTP status.
// The error code is derived from the HTTP status (e.g. 400 →
// "bad_request"); use writeErrorCode when a more specific slug is
// needed (e.g. "insufficient_credits" instead of generic
// "payment_required").
func writeError(w http.ResponseWriter, status int, message string) {
	writeErrorCode(w, status, defaultCodeForStatus(status), message)
}

// writeErrorWithDetails is the extended form of writeError for
// handlers that need to ship additional structured fields alongside
// the unified envelope — e.g. the gauntlet credits gate has to surface
// the user's current balance and required credit count so the
// frontend can render an actionable "earn or wait" prompt.
//
// Details are nested under `error.details` (NOT merged at the top
// level) so the envelope shape stays stable regardless of which
// fields a handler ships.
func writeErrorWithDetails(w http.ResponseWriter, status int, message string, details map[string]any) {
	writeErrorCodeWithDetails(w, status, defaultCodeForStatus(status), message, details)
}

// writeErrorCode is the explicit-code variant of writeError. Handlers
// that have a domain-specific error slug (e.g. "csrf_invalid",
// "rate_limited", "insufficient_credits") should use this form so
// clients can switch on `error.code` without string-matching the HTTP
// status.
func writeErrorCode(w http.ResponseWriter, status int, code, message string) {
	writeErrorCodeWithDetails(w, status, code, message, nil)
}

// writeErrorCodeWithDetails is the most explicit form: caller supplies
// both the code slug AND any structured details. All other writers
// fan into this one so the wire shape stays in a single place.
func writeErrorCodeWithDetails(w http.ResponseWriter, status int, code, message string, details map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	reqID := w.Header().Get(RequestIDHeader)
	w.WriteHeader(status)

	body := ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
		RequestID: reqID,
		Status:    status,
	}
	if len(details) > 0 {
		body.Error.Details = details
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("hexapi writeError encode: %v", err)
	}
}

// defaultCodeForStatus maps an HTTP status code to a stable
// snake-case slug derived from http.StatusText. Unknown statuses
// fall back to "error_<status>" so the field is never empty.
func defaultCodeForStatus(status int) string {
	text := http.StatusText(status)
	if text == "" {
		return statusFallbackCode(status)
	}
	// "Bad Request" → "bad_request"; "I'm a teapot" → "im_a_teapot".
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ', r == '-':
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return statusFallbackCode(status)
	}
	return b.String()
}

func statusFallbackCode(status int) string {
	return "error_" + strconv.Itoa(status)
}
