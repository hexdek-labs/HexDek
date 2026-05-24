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
// ErrorResponse with both fields populated, and the HTTP status on the
// response matches the status field in the body.
func TestWriteError_ShapeAndHeaders(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		message string
	}{
		{"bad request", http.StatusBadRequest, "missing card name"},
		{"forbidden", http.StatusForbidden, "forbidden"},
		{"not found", http.StatusNotFound, "card not found"},
		{"internal", http.StatusInternalServerError, "list: db closed"},
		{"too many", http.StatusTooManyRequests, "clone rate limit exceeded (max 5 per hour)"},
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
			if body.Error != tc.message {
				t.Errorf("body.Error: want %q, got %q", tc.message, body.Error)
			}
			if body.Status != tc.status {
				t.Errorf("body.Status: want %d, got %d", tc.status, body.Status)
			}
		})
	}
}

// TestWriteError_NoTrailingHeaders verifies that writeError commits
// the status before writing the body. Regression for the pre-r60 bug
// where two handlers in handler.go called writeJSON before
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

// TestErrorResponse_JSONSchema pins the public JSON field names so
// existing frontend / third-party clients don't break on a rename.
func TestErrorResponse_JSONSchema(t *testing.T) {
	payload, err := json.Marshal(ErrorResponse{Error: "boom", Status: 500})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(payload)
	if !strings.Contains(s, `"error":"boom"`) {
		t.Errorf("expected \"error\" field, got %s", s)
	}
	if !strings.Contains(s, `"status":500`) {
		t.Errorf("expected \"status\" field, got %s", s)
	}
}
