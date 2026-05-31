package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Pins the spectator lineage handler onto the unified ErrorResponse
// envelope. Pre-r60 the handler returned plain-text via http.Error,
// breaking the contract every other hexapi handler upholds (nested
// {error: {code, message}} body + Content-Type: application/json +
// X-Content-Type-Options: nosniff). All three error branches
// (bad-format 400, rooms-unavailable 503, not-found 404) must decode
// as ErrorResponse with the expected code slug.
func TestSpectatorLineage_ErrorEnvelope_R60(t *testing.T) {
	cases := []struct {
		name       string
		setup      func() (*Showmatch, string)
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name: "bad format → 400 bad_request",
			setup: func() (*Showmatch, string) {
				return &Showmatch{rooms: NewRoomManager()}, "notavalidid"
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
			wantMsg:    "invalid instance_id",
		},
		{
			name: "rooms unavailable → 503 service_unavailable",
			setup: func() (*Showmatch, string) {
				return &Showmatch{}, "h0OGVW20042"
			},
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "service_unavailable",
			wantMsg:    "spectator rooms unavailable",
		},
		{
			name: "not found → 404 not_found",
			setup: func() (*Showmatch, string) {
				return &Showmatch{rooms: NewRoomManager()}, "h0OGVW20999"
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantMsg:    "instance_id not found",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			sm, id := tc.setup()
			req := httptest.NewRequest(http.MethodGet, "/api/spectator/lineage/"+id, nil)
			req.SetPathValue("instance_id", id)
			w := httptest.NewRecorder()
			sm.handleSpectatorLineage(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%s)", tc.wantStatus, w.Code, w.Body.String())
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type: want application/json, got %q", ct)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("X-Content-Type-Options: want nosniff, got %q", w.Header().Get("X-Content-Type-Options"))
			}

			var body ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, w.Body.String())
			}
			if body.Error.Code != tc.wantCode {
				t.Errorf("error.code: want %q, got %q", tc.wantCode, body.Error.Code)
			}
			if body.Error.Message != tc.wantMsg {
				t.Errorf("error.message: want %q, got %q", tc.wantMsg, body.Error.Message)
			}
			if body.Status != tc.wantStatus {
				t.Errorf("body.status: want %d, got %d", tc.wantStatus, body.Status)
			}
		})
	}
}
