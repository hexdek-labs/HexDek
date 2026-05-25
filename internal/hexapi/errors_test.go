package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWriteError_ShapeAndHeaders pins the unified error contract:
// Content-Type is application/json, the body decodes into
// ErrorResponse with nested code+message populated, and the HTTP
// status on the response matches the status field in the body.
func TestWriteError_ShapeAndHeaders(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		message  string
		wantCode string
	}{
		{"bad request", http.StatusBadRequest, "missing card name", "bad_request"},
		{"forbidden", http.StatusForbidden, "forbidden", "forbidden"},
		{"not found", http.StatusNotFound, "card not found", "not_found"},
		{"internal", http.StatusInternalServerError, "list: db closed", "internal_server_error"},
		{"too many", http.StatusTooManyRequests, "clone rate limit exceeded (max 5 per hour)", "too_many_requests"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			writeError(rr, tc.status, tc.message)

			if rr.Code != tc.status {
				t.Errorf("status: want %d, got %d", tc.status, rr.Code)
			}
			ct := rr.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type: want application/json, got %q", ct)
			}
			if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("X-Content-Type-Options: want nosniff, got %q", rr.Header().Get("X-Content-Type-Options"))
			}

			var body ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rr.Body.String())
			}
			if body.Error.Message != tc.message {
				t.Errorf("body.error.message: want %q, got %q", tc.message, body.Error.Message)
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("body.error.code: want %q, got %q", tc.wantCode, body.Error.Code)
			}
			if body.Status != tc.status {
				t.Errorf("body.status: want %d, got %d", tc.status, body.Status)
			}
		})
	}
}

// TestWriteError_StatusCommittedBeforeBody verifies that writeError
// commits the status before writing the body. Regression for the
// pre-r60 bug where two handlers in handler.go called writeJSON before
// WriteHeader, causing the status to silently default to 200.
func TestWriteError_StatusCommittedBeforeBody(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusBadGateway, "upstream failed")

	if rr.Code == http.StatusOK {
		t.Fatalf("status defaulted to 200 — body write happened before WriteHeader")
	}
	if rr.Code != http.StatusBadGateway {
		t.Errorf("status: want 502, got %d", rr.Code)
	}
}

// TestWriteErrorWithDetails_NestedDetails pins the extended-envelope
// contract: handlers that need additional structured fields alongside
// `error.code` + `error.message` (e.g. the gauntlet credits gate which
// surfaces balance / needed / quota) get them nested under
// `error.details` so the envelope shape stays stable regardless of
// which fields a handler ships.
func TestWriteErrorWithDetails_NestedDetails(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorCodeWithDetails(rr, http.StatusPaymentRequired, "insufficient_credits", "not enough credits", map[string]any{
		"balance": 7,
		"needed":  5,
	})

	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("status: want 402, got %d", rr.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rr.Body.String())
	}
	if body.Error.Code != "insufficient_credits" {
		t.Errorf("body.error.code: want %q, got %q", "insufficient_credits", body.Error.Code)
	}
	if body.Error.Message != "not enough credits" {
		t.Errorf("body.error.message: want %q, got %q", "not enough credits", body.Error.Message)
	}
	if body.Status != http.StatusPaymentRequired {
		t.Errorf("body.status: want 402, got %d", body.Status)
	}

	details, ok := body.Error.Details.(map[string]any)
	if !ok {
		t.Fatalf("body.error.details: want map, got %T (%v)", body.Error.Details, body.Error.Details)
	}
	if got, _ := details["balance"].(float64); int(got) != 7 {
		t.Errorf("details.balance: want 7, got %v", details["balance"])
	}
	if got, _ := details["needed"].(float64); int(got) != 5 {
		t.Errorf("details.needed: want 5, got %v", details["needed"])
	}
}

// TestWriteErrorWithDetails_NilDetailsOmitsField pins the degenerate
// case: a handler that has no extras to share should still be safe to
// call this helper, AND the wire body must NOT include a null/empty
// "details" field. Decoders that only know about ErrorBody.Code +
// Message stay clean.
func TestWriteErrorWithDetails_NilDetailsOmitsField(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorWithDetails(rr, http.StatusBadRequest, "missing field", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
	raw := rr.Body.String()
	if strings.Contains(raw, `"details"`) {
		t.Errorf("nil details should be omitted from wire body, got %q", raw)
	}
	var body ErrorResponse
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, raw)
	}
	if body.Error.Message != "missing field" || body.Status != http.StatusBadRequest {
		t.Errorf("body: want {missing field, 400}, got %+v", body)
	}
}

// TestWriteErrorCode_ExplicitCodeOverridesStatusDerived pins that
// handlers with a domain-specific slug (e.g. csrf_invalid,
// insufficient_credits) get their code emitted verbatim, not
// overwritten by the http.StatusText-derived default.
func TestWriteErrorCode_ExplicitCodeOverridesStatusDerived(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorCode(rr, http.StatusForbidden, "csrf_invalid", "bad token")

	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "csrf_invalid" {
		t.Errorf("body.error.code: want csrf_invalid, got %q", body.Error.Code)
	}
	if body.Error.Message != "bad token" {
		t.Errorf("body.error.message: want %q, got %q", "bad token", body.Error.Message)
	}
}

// TestErrorResponse_JSONSchema pins the public JSON field names so
// existing frontend / third-party clients don't break on a rename.
func TestErrorResponse_JSONSchema(t *testing.T) {
	payload, err := json.Marshal(ErrorResponse{
		Error:     ErrorBody{Code: "internal_server_error", Message: "boom"},
		RequestID: "abc123",
		Status:    500,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(payload)
	for _, want := range []string{
		`"error":{`,
		`"code":"internal_server_error"`,
		`"message":"boom"`,
		`"request_id":"abc123"`,
		`"status":500`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("expected substring %q in payload, got %s", want, s)
		}
	}
}

// TestErrorResponse_RequestIDRoundTrips pins the integration: when
// RequestIDMiddleware has set X-Request-Id on the response headers
// before the handler runs, writeError must pull it back and include
// it in the body so error logs from the browser console can be
// correlated against server logs without a separate trace fetch.
func TestErrorResponse_RequestIDRoundTrips(t *testing.T) {
	rr := httptest.NewRecorder()
	rr.Header().Set(RequestIDHeader, "test-trace-deadbeef")

	writeError(rr, http.StatusBadRequest, "bad input")

	if got := rr.Header().Get(RequestIDHeader); got != "test-trace-deadbeef" {
		t.Errorf("response X-Request-Id header: want test-trace-deadbeef, got %q", got)
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.RequestID != "test-trace-deadbeef" {
		t.Errorf("body.request_id: want test-trace-deadbeef, got %q", body.RequestID)
	}
}

// TestDefaultCodeForStatus pins the http.StatusText → snake_case
// derivation for the codes that handlers most commonly emit.
func TestDefaultCodeForStatus(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusBadRequest, "bad_request"},
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not_found"},
		{http.StatusMethodNotAllowed, "method_not_allowed"},
		{http.StatusConflict, "conflict"},
		{http.StatusPaymentRequired, "payment_required"},
		{http.StatusTooManyRequests, "too_many_requests"},
		{http.StatusInternalServerError, "internal_server_error"},
		{http.StatusBadGateway, "bad_gateway"},
		{http.StatusServiceUnavailable, "service_unavailable"},
		{http.StatusGatewayTimeout, "gateway_timeout"},
		{499, "error_499"}, // unknown status falls back
	}
	for _, tc := range cases {
		if got := defaultCodeForStatus(tc.status); got != tc.want {
			t.Errorf("defaultCodeForStatus(%d): want %q, got %q", tc.status, tc.want, got)
		}
	}
}
