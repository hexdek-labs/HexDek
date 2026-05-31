package hexapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// csrf_audit_r60_test.go — pins the r60 CSRF audit additions:
//   - RequireCSRF bypasses OPTIONS preflights (CORS-correctness fix)
//   - 403 body decodes as the unified ErrorResponse envelope with a
//     stable `csrf_invalid` code slug that collapses all four sentinel
//     reasons (missing / malformed / expired / bad signature)
//   - Newly wrapped routes 403 without a token (closes the prior
//     audit gap: PATCH curse, gauntlet start, live speed, spectate
//     spawn, webhook register, webhook delete)
//   - Showmatch CSRFStore late-binding works after registration

// --- token-present / token-absent / token-invalid / GET-no-token / OPTIONS ---

// scenario 1: token-present → wrapped handler executes.
func TestRequireCSRF_TokenPresentValid_HandlerRuns(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, time.Hour)
	tok, _, _ := store.IssueToken()

	called := false
	h := RequireCSRF(store, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set(CSRFHeader, tok)
	w := httptest.NewRecorder()
	h(w, r)
	if !called {
		t.Fatal("valid token: handler must run")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", w.Code)
	}
}

// scenario 2: token-absent → 403 with unified envelope, code=csrf_invalid.
func TestRequireCSRF_TokenAbsent_403WithUnifiedEnvelope(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, time.Hour)
	h := RequireCSRF(store, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("absent-token: handler must NOT run")
	})
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	w := httptest.NewRecorder()
	h(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	if nosniff := w.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("X-Content-Type-Options=%q, want nosniff", nosniff)
	}
	var body ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, w.Body.String())
	}
	if body.Error.Code != "csrf_invalid" {
		t.Errorf("error.code=%q, want csrf_invalid", body.Error.Code)
	}
	if !strings.Contains(body.Error.Message, "csrf") {
		t.Errorf("error.message=%q must mention csrf", body.Error.Message)
	}
	if body.Status != http.StatusForbidden {
		t.Errorf("body.status=%d, want 403", body.Status)
	}
}

// scenario 3: token-invalid covers all four sentinel reasons. All MUST
// collapse to one slug — the wire must not let an attacker probe which
// half of the token validator their fake tripped.
func TestRequireCSRF_TokenInvalid_AllSentinelsCollapseToOneSlug(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, time.Hour)
	valid, _, _ := store.IssueToken()

	cases := []struct {
		name string
		tok  string
	}{
		{"malformed not-base64", "!!!not-base64!!!"},
		{"malformed wrong length", "YWJj"}, // valid base64, wrong size
		{"bad signature tampered tail", valid[:len(valid)-4] + "AAAA"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			h := RequireCSRF(store, func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("invalid token: handler must NOT run")
			})
			r := httptest.NewRequest(http.MethodPost, "/x", nil)
			r.Header.Set(CSRFHeader, tc.tok)
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want 403", w.Code)
			}
			var body ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != "csrf_invalid" {
				t.Errorf("error.code=%q, want csrf_invalid (must collapse all reasons)", body.Error.Code)
			}
		})
	}
}

// scenario 3b: expired token also collapses to csrf_invalid. Uses the
// injectable clock to age the token past TTL without sleeping.
func TestRequireCSRF_TokenExpired_CollapsesToCSRFInvalid(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, time.Minute)
	now := time.Now()
	store.now = func() time.Time { return now }
	tok, _, _ := store.IssueToken()

	// Advance past TTL.
	now = now.Add(2 * time.Minute)

	h := RequireCSRF(store, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("expired token: handler must NOT run")
	})
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	r.Header.Set(CSRFHeader, tok)
	w := httptest.NewRecorder()
	h(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", w.Code)
	}
	var body ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "csrf_invalid" {
		t.Errorf("expired token error.code=%q, want csrf_invalid", body.Error.Code)
	}
}

// scenario 4: GET requests on a CSRF-wrapped path don't need a token.
// In practice the mux routes GET to a different handler, but the
// explicit method-check in RequireCSRF defends against a misroute.
// Pin via the Handler.Register path so we exercise the real mux too.
func TestRequireCSRF_GETRoutesNoTokenNeeded(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h := &Handler{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	h.Register(mux)

	for _, route := range []string{
		"GET /api/decks",
		"GET /api/csrf",
		"GET /api/meta",
	} {
		parts := strings.SplitN(route, " ", 2)
		req := httptest.NewRequest(parts[0], parts[1], nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
			t.Errorf("%s: GET should not require CSRF, got 403", route)
		}
	}
}

// scenario 5: OPTIONS preflight bypasses CSRF. Browsers issuing a CORS
// preflight do NOT include the X-CSRF-Token header (only listed
// headers travel in preflight) — a 403 here would block the real
// request from ever firing. Pin so a refactor doesn't accidentally
// regress to "preflight 403s every cross-origin write."
func TestRequireCSRF_OPTIONSPreflightBypass(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	store := NewCSRFStore(secret, time.Hour)
	called := false
	h := RequireCSRF(store, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// OPTIONS preflight with NO X-CSRF-Token (matches real browser shape).
	r := httptest.NewRequest(http.MethodOptions, "/x", nil)
	r.Header.Set("Origin", "https://hexdek.dev")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", CSRFHeader)
	w := httptest.NewRecorder()
	h(w, r)

	if !called {
		t.Fatal("OPTIONS preflight: handler must run (CSRF bypassed by design)")
	}
	if w.Code == http.StatusForbidden {
		t.Errorf("OPTIONS preflight returned 403; would block all cross-origin writes")
	}
}

// --- newly wrapped routes (gap closure) ----------------------------------

// PATCH /api/decks/{owner}/{id}/curse — was the clearest oversight,
// sits as a sibling to the wrapped PATCH /api/decks/{owner}/{id}.
// Now wrapped via sm.requireCSRFLate. Test exercises the real mux so
// late-binding is included in the coverage.
func TestCSRFGap_PatchCurse_403WithoutToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	sm := &Showmatch{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	req := httptest.NewRequest(http.MethodPatch, "/api/decks/alice/voltron/curse",
		strings.NewReader(`{"curse_text":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("PATCH curse without token: status=%d, want 403 (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "csrf") {
		t.Errorf("body should mention csrf, got %q", w.Body.String())
	}
}

// POST /api/gauntlet/{owner}/{id} — high-cost tournament action.
func TestCSRFGap_StartGauntlet_403WithoutToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	sm := &Showmatch{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/gauntlet/alice/voltron", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST gauntlet without token: status=%d, want 403", w.Code)
	}
}

// POST /api/live/speed — live game control mutation.
func TestCSRFGap_LiveSpeed_403WithoutToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	sm := &Showmatch{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/live/speed?multiplier=10", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST live/speed without token: status=%d, want 403", w.Code)
	}
}

// POST /api/spectate/spawn — creates a room (spawns a game-driver
// goroutine).
func TestCSRFGap_SpawnSpectateRoom_403WithoutToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	sm := &Showmatch{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/spectate/spawn",
		strings.NewReader(`{"deck_id":"alice/voltron"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("POST spectate/spawn without token: status=%d, want 403", w.Code)
	}
}

// POST /api/webhooks + DELETE /api/webhooks/{id} — webhook registration
// and removal are state-changing user operations.
func TestCSRFGap_WebhookRegisterDelete_403WithoutToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h := &Handler{CSRFStore: NewCSRFStore(secret, time.Hour)}
	mux := http.NewServeMux()
	h.RegisterWebhookRoutes(mux)

	cases := []struct {
		method, path string
		body         string
	}{
		{http.MethodPost, "/api/webhooks", `{"url":"https://example.com/hook"}`},
		{http.MethodDelete, "/api/webhooks/abc123", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var req *http.Request
			if tc.body != "" {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want 403 (body=%q)", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "csrf") {
				t.Errorf("body should mention csrf, got %q", w.Body.String())
			}
		})
	}
}

// Late-binding integration: sm.CSRFStore can be set AFTER
// RegisterShowmatch runs. Pre-r60 the wrapper captured the store at
// registration time, so this pattern would always 200 (store seen as
// nil → pass-through). The late-binding fix resolves sm.CSRFStore
// per-request — pin so a refactor back to early-binding gets caught.
func TestCSRFGap_ShowmatchLateBinding_StoreSetAfterRegister(t *testing.T) {
	sm := &Showmatch{} // no store yet
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	// Set store AFTER routes are registered. This mirrors the
	// cmd/hexdek-server/main.go ordering where sm.RegisterShowmatch
	// runs before hexAPI.CSRFStore is constructed.
	secret, _ := RandomCSRFSecret()
	sm.CSRFStore = NewCSRFStore(secret, time.Hour)

	req := httptest.NewRequest(http.MethodPatch, "/api/decks/alice/voltron/curse",
		strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("late-bound store: PATCH curse without token must 403, got %d (body=%q)",
			w.Code, w.Body.String())
	}
}

// Late-binding integration: sm.CSRFStore stays nil (e.g. dev binary
// without HEXDEK_CSRF_ENFORCE) — wrapper is a pass-through.
func TestCSRFGap_ShowmatchLateBinding_NilStoreIsPassThrough(t *testing.T) {
	sm := &Showmatch{} // no store, never set
	mux := http.NewServeMux()
	sm.RegisterShowmatch(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/live/speed?multiplier=1.0", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code == http.StatusForbidden && strings.Contains(w.Body.String(), "csrf") {
		t.Errorf("nil store should pass through, got 403 csrf")
	}
}
