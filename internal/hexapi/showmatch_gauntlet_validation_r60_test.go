package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// r60 — pin path-component validation on the gauntlet handlers.
//
// Bug history: handleStartGauntlet and handleGetGauntlet both built
// deckKey := owner + "/" + id directly from r.PathValue without
// running it through validatePathComponent. Every other deck-
// targeting hexapi handler (handleDeckSharePage, handleDeckUpgrade,
// handleDeleteDeck, ...) already runs that guard. deckKey is used as
// a credit-ledger reference, a log-line interpolation, and an SSE
// broadcast topic, so a path-traversal owner like ".." or an id with
// a slash could pollute the credits ledger and the SSE topic map and
// inject newlines into log lines.

// TestHandleStartGauntlet_PathValidation pins the 400 + unified
// ErrorResponse contract for malformed owner / id. Valid input is NOT
// exercised here — that requires a real Showmatch fixture with a
// deck pool, credits store, and the gauntletSem goroutine; covered
// separately by integration tests once that harness lands.
func TestHandleStartGauntlet_PathValidation(t *testing.T) {
	sm := &Showmatch{} // bare; validation runs before any state touch

	cases := []struct {
		name       string
		owner      string
		id         string
		wantStatus int
		wantErrMsg string
	}{
		{
			name:       "owner with .. rejected (path traversal)",
			owner:      "..",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "id with slash rejected (deckKey injection)",
			owner:      "alice",
			id:         "korvold/extra",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "owner with NUL byte rejected",
			owner:      "alice\x00",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "id with backslash rejected (Windows-style traversal)",
			owner:      "alice",
			id:         "korvold\\evil",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "empty owner rejected",
			owner:      "",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "empty id rejected",
			owner:      "alice",
			id:         "",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "owner with newline injection rejected",
			owner:      "alice\nfake-log-line",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost,
				"/api/showmatch/gauntlet/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id),
				strings.NewReader(""))
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			sm.handleStartGauntlet(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			var body ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
			}
			if body.Error != tc.wantErrMsg {
				t.Errorf("body.error: want %q, got %q", tc.wantErrMsg, body.Error)
			}
			if body.Status != tc.wantStatus {
				t.Errorf("body.status: want %d, got %d", tc.wantStatus, body.Status)
			}
			// Critical: when validation rejects, no gauntlet result
			// should have been registered (no map write, no SSE
			// broadcast, no log injection downstream).
			sm.gauntletMu.RLock()
			n := len(sm.gauntlets)
			sm.gauntletMu.RUnlock()
			if n != 0 {
				t.Errorf("gauntlets map should be empty after rejected input, got %d entries", n)
			}
		})
	}
}

// TestHandleGetGauntlet_PathValidation pins the same contract on the
// read endpoint. The error path is identical; the happy path returns
// 200 + {"status":"none"} when no gauntlet has been registered, so we
// can also exercise the post-validation flow with a bare Showmatch.
func TestHandleGetGauntlet_PathValidation(t *testing.T) {
	sm := &Showmatch{}

	cases := []struct {
		name       string
		owner      string
		id         string
		wantStatus int
		wantErrMsg string // "" for happy path
		wantStatus2 string // body "status" field on happy path
	}{
		{
			name:        "happy path — valid input, no gauntlet registered yet",
			owner:       "alice",
			id:          "korvold",
			wantStatus:  http.StatusOK,
			wantStatus2: "none",
		},
		{
			name:       "owner with .. rejected",
			owner:      "..",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "id with slash rejected",
			owner:      "alice",
			id:         "korvold/extra",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
		{
			name:       "empty owner rejected",
			owner:      "",
			id:         "korvold",
			wantStatus: http.StatusBadRequest,
			wantErrMsg: "invalid owner or id",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/api/showmatch/gauntlet/"+url.PathEscape(tc.owner)+"/"+url.PathEscape(tc.id),
				nil)
			req.SetPathValue("owner", tc.owner)
			req.SetPathValue("id", tc.id)
			rr := httptest.NewRecorder()

			sm.handleGetGauntlet(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rr.Code, rr.Body.String())
			}
			if tc.wantErrMsg != "" {
				var body ErrorResponse
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("decode ErrorResponse: %v (raw=%q)", err, rr.Body.String())
				}
				if body.Error != tc.wantErrMsg {
					t.Errorf("body.error: want %q, got %q", tc.wantErrMsg, body.Error)
				}
				if body.Status != tc.wantStatus {
					t.Errorf("body.status: want %d, got %d", tc.wantStatus, body.Status)
				}
				return
			}
			// Happy path — must be JSON with status field.
			var body map[string]any
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v (raw=%q)", err, rr.Body.String())
			}
			if got, _ := body["status"].(string); got != tc.wantStatus2 {
				t.Errorf("body.status: want %q, got %q", tc.wantStatus2, got)
			}
		})
	}
}
