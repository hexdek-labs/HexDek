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

// TestWriteErrorWithDetails_MergesExtras pins the extended-envelope
// contract: handlers that need additional structured fields alongside
// `error` + `status` (e.g. the gauntlet credits gate which surfaces
// balance / needed / quota) get them merged in without breaking the
// base contract. Decoders that only know ErrorResponse must still
// pull a populated Error + Status pair, AND callers that look for
// the extras must find them at the top level.
func TestWriteErrorWithDetails_MergesExtras(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorWithDetails(rr, http.StatusPaymentRequired, "insufficient_credits", map[string]any{
		"balance": 7,
		"needed":  5,
	})

	if rr.Code != http.StatusPaymentRequired {
		t.Errorf("status: want 402, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: want application/json, got %q", ct)
	}
	if rr.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", rr.Header().Get("X-Content-Type-Options"))
	}

	// Base ErrorResponse fields must be present.
	var base ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &base); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, rr.Body.String())
	}
	if base.Error != "insufficient_credits" {
		t.Errorf("body.error: want %q, got %q", "insufficient_credits", base.Error)
	}
	if base.Status != http.StatusPaymentRequired {
		t.Errorf("body.status: want 402, got %d", base.Status)
	}

	// Extras must round-trip too.
	var full map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &full); err != nil {
		t.Fatalf("full body decode: %v", err)
	}
	if got, _ := full["balance"].(float64); int(got) != 7 {
		t.Errorf("balance: want 7, got %v", full["balance"])
	}
	if got, _ := full["needed"].(float64); int(got) != 5 {
		t.Errorf("needed: want 5, got %v", full["needed"])
	}
}

// TestWriteErrorWithDetails_DropsReservedKeys verifies that callers
// cannot overwrite the unified `error` / `status` discriminator from
// the details map. The unified contract wins so frontend decoders
// remain stable even if a handler accidentally passes a clashing key.
func TestWriteErrorWithDetails_DropsReservedKeys(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorWithDetails(rr, http.StatusForbidden, "forbidden", map[string]any{
		"error":  "OVERRIDDEN", // must be ignored
		"status": 999,           // must be ignored
		"reason": "policy_x",
	})

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, rr.Body.String())
	}
	if got, _ := body["error"].(string); got != "forbidden" {
		t.Errorf("error: caller override leaked, want %q got %q", "forbidden", got)
	}
	if got, _ := body["status"].(float64); int(got) != http.StatusForbidden {
		t.Errorf("status: caller override leaked, want %d got %v", http.StatusForbidden, body["status"])
	}
	if got, _ := body["reason"].(string); got != "policy_x" {
		t.Errorf("reason: want %q, got %q", "policy_x", got)
	}
}

// TestWriteErrorWithDetails_NilDetailsBehavesLikeWriteError pins the
// degenerate case: a handler that has no extras to share should still
// be safe to call this helper (it just becomes a writeError
// equivalent), so we don't have to keep two paths in sync.
func TestWriteErrorWithDetails_NilDetailsBehavesLikeWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeErrorWithDetails(rr, http.StatusBadRequest, "missing field", nil)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rr.Code)
	}
	var body ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "missing field" || body.Status != http.StatusBadRequest {
		t.Errorf("body: want {missing field, 400}, got %+v", body)
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
