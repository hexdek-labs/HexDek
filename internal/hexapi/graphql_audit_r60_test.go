package hexapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// graphql_audit_r60_test.go — pins the r60 GraphQL hardening:
//   - POST /graphql is wrapped in RequireCSRF (closes the mutation-
//     bypass gap; importDeck/deleteDeck previously sailed past the
//     CSRF gate that REST mutations enforce)
//   - GET /graphql stays ungated (queries-over-GET aren't part of the
//     CSRF threat model)
//   - OPTIONS preflight bypasses the gate (CORS correctness)
//   - 403 body matches the unified ErrorResponse envelope from PR #778
//   - Per-IP rate limit gate on serve() throttles bursts before query
//     parsing — defends against GraphQL's resolver-fan-out cost
//   - All gates are nil-safe so the existing POC tests (which build
//     GraphQLHandler without setting CSRFStore / Limiter) keep working

// --- CSRF gating on POST /graphql -----------------------------------------

// Reaches /graphql through a real mux (vs the existing helpers that
// call h.serve directly and bypass the mux-level wrapper). Pre-r60
// this would have returned 200 with a mutation result; post-r60 the
// CSRF wrapper short-circuits at 403.
func TestGraphQL_POST_RequiresCSRFToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h, err := NewGraphQLHandler(seedShowmatch(t), "")
	if err != nil {
		t.Fatalf("NewGraphQLHandler: %v", err)
	}
	h.CSRFStore = NewCSRFStore(secret, time.Hour)

	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	body, _ := json.Marshal(graphqlRequest{Query: `{ games { id } }`})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("POST without CSRF token must 403, got %d (body=%q)", w.Code, w.Body.String())
	}
	// Unified envelope: csrf_invalid code + nosniff + json content-type.
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	var er ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&er); err != nil {
		t.Fatalf("body did not decode as ErrorResponse: %v (raw=%q)", err, w.Body.String())
	}
	if er.Error.Code != "csrf_invalid" {
		t.Errorf("error.code=%q, want csrf_invalid", er.Error.Code)
	}
}

// Valid token → POST runs through to the resolver. Uses the schema
// introspection query so we don't need any deck/fs fixtures.
func TestGraphQL_POST_ValidTokenPasses(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.CSRFStore = NewCSRFStore(secret, time.Hour)

	tok, _, _ := h.CSRFStore.IssueToken()
	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(CSRFHeader, tok)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("valid token: status=%d, want 200 (body=%q)", w.Code, w.Body.String())
	}
	// Body should be a GraphQL envelope (data populated, no errors).
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp["data"] == nil {
		t.Errorf("data missing — wrapper might have eaten the body: %v", resp)
	}
}

// GET /graphql (browser-friendly ad-hoc query form, ?query=...) is
// NOT CSRF-gated. Pin so a refactor that wraps both methods doesn't
// silently break the dev/tooling path that uses GET.
func TestGraphQL_GET_DoesNotRequireCSRFToken(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.CSRFStore = NewCSRFStore(secret, time.Hour)
	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	req := httptest.NewRequest(http.MethodGet, "/graphql?query={__schema{queryType{name}}}", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET without token must pass (queries-over-GET aren't CSRF-gated), got %d (body=%q)",
			w.Code, w.Body.String())
	}
}

// OPTIONS preflight bypasses the CSRF gate — browsers don't send the
// custom X-CSRF-Token header in preflight, so a 403 here would break
// every cross-origin GraphQL request before the real POST fires.
// Matches the bypass behavior pinned for REST in PR #843.
func TestGraphQL_OPTIONSPreflightBypasses(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.CSRFStore = NewCSRFStore(secret, time.Hour)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /graphql", RequireCSRF(h.CSRFStore, h.serve))
	mux.HandleFunc("OPTIONS /graphql", RequireCSRF(h.CSRFStore, h.serve))

	req := httptest.NewRequest(http.MethodOptions, "/graphql", nil)
	req.Header.Set("Origin", "https://hexdek.dev")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", CSRFHeader)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("OPTIONS preflight 403'd; would block cross-origin GraphQL: body=%q", w.Body.String())
	}
}

// Nil CSRFStore = pass-through. Defends the existing POC tests that
// build GraphQLHandler without setting the store.
func TestGraphQL_POST_NilCSRFStoreIsPassThrough(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	// h.CSRFStore stays nil.
	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("nil CSRFStore should pass through; status=%d body=%q", w.Code, w.Body.String())
	}
}

// --- Rate limit gating ----------------------------------------------------

// Per-IP token-bucket throttles a burst from one address. The gate
// runs BEFORE query parsing so a malformed/expensive query doesn't
// cost the limiter anything extra. With CSRF disabled (nil store) so
// we can exercise the rate-limit path without minting a token.
func TestGraphQL_RateLimit_PerIPBurstThen429(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.Limiter = NewRateLimiter(2, 1.0/60.0) // 2-burst, 1/min refill

	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})

	// 2 requests from one IP pass.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "203.0.113.7")
		w := httptest.NewRecorder()
		h.serve(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("burst %d should pass: status=%d body=%q", i+1, w.Code, w.Body.String())
		}
	}
	// 3rd from same IP: 429.
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	w := httptest.NewRecorder()
	h.serve(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd burst must 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("429 missing Retry-After header")
	}
	// 429 body uses the unified envelope with too_many_requests code.
	var er ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&er); err != nil {
		t.Fatalf("429 body did not decode as ErrorResponse: %v (raw=%q)", err, w.Body.String())
	}
	if er.Error.Code != "too_many_requests" {
		t.Errorf("error.code=%q, want too_many_requests", er.Error.Code)
	}
	if !strings.Contains(er.Error.Message, "graphql") {
		t.Errorf("error.message=%q must carry the graphql label", er.Error.Message)
	}
}

// Independent IPs have independent buckets — verifies per-IP keying
// isolates attackers from legitimate traffic.
func TestGraphQL_RateLimit_IndependentIPsIsolated(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.Limiter = NewRateLimiter(2, 1.0/60.0)

	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})

	// Burn attacker's budget.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "10.0.0.99")
		w := httptest.NewRecorder()
		h.serve(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("attacker burst %d: %d", i+1, w.Code)
		}
	}
	// Confirm attacker is now throttled.
	{
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "10.0.0.99")
		w := httptest.NewRecorder()
		h.serve(w, req)
		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("attacker 3rd: expected 429 got %d", w.Code)
		}
	}
	// Legitimate user from different IP passes.
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.100")
	w := httptest.NewRecorder()
	h.serve(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("legitimate user from independent IP should pass, got %d: %s",
			w.Code, w.Body.String())
	}
}

// Nil Limiter = no throttling. Defends the existing POC tests.
func TestGraphQL_RateLimit_NilLimiterIsPassThrough(t *testing.T) {
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	// Limiter nil.
	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})

	// Spam 10 from same IP — none throttle.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "1.2.3.4")
		w := httptest.NewRecorder()
		h.serve(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("nil limiter request %d: status=%d", i+1, w.Code)
		}
	}
}

// Rate-limit fires before CSRF — verifies a malformed-token burst
// can't evade the limiter by tripping the CSRF gate first. Both
// gates active: limiter is checked AT THE HANDLER (inside h.serve);
// CSRF is checked AT THE MUX (RegisterGraphQL wraps serve). Ordering
// is therefore CSRF → handler-rate-limit, NOT the other way around.
// So a burst of bad-token POSTs gets 403'd by CSRF and never touches
// the limiter — pin that ordering so it's intentional.
func TestGraphQL_GateOrdering_CSRFRunsBeforeRateLimit(t *testing.T) {
	secret, _ := RandomCSRFSecret()
	h, _ := NewGraphQLHandler(seedShowmatch(t), "")
	h.CSRFStore = NewCSRFStore(secret, time.Hour)
	h.Limiter = NewRateLimiter(1, 1.0/60.0) // 1-burst — would fire on 2nd if reached

	mux := http.NewServeMux()
	h.RegisterGraphQL(mux)

	body, _ := json.Marshal(graphqlRequest{Query: `{ __schema { queryType { name } } }`})

	// 5 bad-token bursts from same IP. Each gets 403 from CSRF; the
	// limiter never sees them. Otherwise we'd see one 403, then 429s.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(CSRFHeader, "bad-token")
		req.Header.Set("X-Forwarded-For", "5.5.5.5")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("bad-token request %d: status=%d, want 403 (if 429 here, CSRF→rate-limit ordering is broken)",
				i+1, w.Code)
		}
	}
	// Now a valid-token request from the same IP: limiter is at full
	// budget (CSRF rejections never charged it) → passes.
	tok, _, _ := h.CSRFStore.IssueToken()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(CSRFHeader, tok)
	req.Header.Set("X-Forwarded-For", "5.5.5.5")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("post-CSRF-rejection valid request must pass with fresh limiter budget, got %d (body=%q)",
			w.Code, w.Body.String())
	}
}
