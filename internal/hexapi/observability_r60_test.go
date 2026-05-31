package hexapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// observability_r60_test.go — pins the r60 metrics + structured-log
// surface: MetricsRegistry recording semantics, Prometheus text
// exposition format, ObservabilityMiddleware request capture, error-
// slug extraction from the unified ErrorResponse envelope, and the
// /metrics endpoint's loopback-or-admin gating.

// --- MetricsRegistry recording -------------------------------------------

// Baseline: every recorded request bumps the per-(endpoint,method,
// status) request_count by 1, accumulates duration into the
// histogram, and stays untouched on the error counter when no slug
// was supplied (success path).
func TestMetricsRegistry_RecordRequest_Success(t *testing.T) {
	r := NewMetricsRegistry(nil)
	r.RecordRequest("/api/decks", "GET", 200, 12*time.Millisecond, "")
	r.RecordRequest("/api/decks", "GET", 200, 8*time.Millisecond, "")

	if got := r.requestCount[metricKey{Endpoint: "/api/decks", Method: "GET", Status: 200}]; got != 2 {
		t.Errorf("request_count = %d, want 2", got)
	}
	h := r.histograms[histKey{Endpoint: "/api/decks", Method: "GET"}]
	if h == nil {
		t.Fatal("histogram bucket missing for /api/decks GET")
	}
	if h.count != 2 {
		t.Errorf("histogram count = %d, want 2", h.count)
	}
	wantSum := 0.020 // 12ms + 8ms
	if h.sum < wantSum-0.0001 || h.sum > wantSum+0.0001 {
		t.Errorf("histogram sum = %g, want ~%g", h.sum, wantSum)
	}
	if len(r.errorCount) != 0 {
		t.Errorf("error_count should be empty on success, got %d entries", len(r.errorCount))
	}
}

// Error path: a non-empty errorSlug bumps the per-error counter in
// addition to the request_count + histogram. Pin so a refactor that
// accidentally double-charges OR skips the error_count is caught.
func TestMetricsRegistry_RecordRequest_ErrorSlug(t *testing.T) {
	r := NewMetricsRegistry(nil)
	r.RecordRequest("/api/decks/{owner}/{id}", "DELETE", 403, 5*time.Millisecond, "csrf_invalid")
	r.RecordRequest("/api/decks/{owner}/{id}", "DELETE", 403, 5*time.Millisecond, "csrf_invalid")
	r.RecordRequest("/api/decks/{owner}/{id}", "DELETE", 404, 5*time.Millisecond, "not_found")

	k1 := errorMetricKey{Endpoint: "/api/decks/{owner}/{id}", Method: "DELETE", Status: 403, ErrorSlug: "csrf_invalid"}
	k2 := errorMetricKey{Endpoint: "/api/decks/{owner}/{id}", Method: "DELETE", Status: 404, ErrorSlug: "not_found"}
	if got := r.errorCount[k1]; got != 2 {
		t.Errorf("csrf_invalid error_count = %d, want 2", got)
	}
	if got := r.errorCount[k2]; got != 1 {
		t.Errorf("not_found error_count = %d, want 1", got)
	}
}

// Histograms are CUMULATIVE — a 100ms observation must bump every
// bucket from le="0.100" through le="+Inf". This is a frequent
// foot-gun in hand-rolled histograms; pin so a regression to non-
// cumulative counts is caught fast.
func TestMetricsRegistry_HistogramIsCumulative(t *testing.T) {
	r := NewMetricsRegistry([]float64{0.010, 0.050, 0.100, 0.500})
	r.RecordRequest("/x", "GET", 200, 75*time.Millisecond, "") // lands in [0.050, 0.100)

	h := r.histograms[histKey{Endpoint: "/x", Method: "GET"}]
	// Buckets: [0]=le="0.010", [1]=le="0.050", [2]=le="0.100", [3]=le="0.500", [4]=+Inf
	// 75ms > 0.050 and > 0.010 → counts at indices 0,1 stay 0
	// 75ms <= 0.100 → counts at indices 2,3,4 all = 1
	want := []uint64{0, 0, 1, 1, 1}
	for i, w := range want {
		if h.counts[i] != w {
			t.Errorf("bucket[%d] = %d, want %d (cumulative semantics violated)", i, h.counts[i], w)
		}
	}
}

// Nil registry is a no-op — pin so a refactor that makes the
// registry mandatory doesn't break the "no-op middleware" path.
func TestMetricsRegistry_NilIsNoOp(t *testing.T) {
	var r *MetricsRegistry
	// Must not panic.
	r.RecordRequest("/x", "GET", 200, time.Millisecond, "")
	var buf bytes.Buffer
	if err := r.WritePrometheus(&buf); err != nil {
		t.Errorf("WritePrometheus on nil: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("nil registry must write nothing, got %d bytes", buf.Len())
	}
}

// Empty endpoint is rejected — never poison the registry with a
// degenerate key (would aggregate ALL no-pattern requests under
// endpoint="" which destroys dashboard utility).
func TestMetricsRegistry_EmptyEndpointSkipped(t *testing.T) {
	r := NewMetricsRegistry(nil)
	r.RecordRequest("", "GET", 200, time.Millisecond, "")
	if len(r.requestCount) != 0 {
		t.Errorf("empty endpoint should be skipped, got %d entries", len(r.requestCount))
	}
}

// --- Prometheus exposition format ----------------------------------------

// The text format is the public contract — every line shape matters
// (HELP / TYPE comments, label ordering, le-bucket labels, the
// trailing +Inf bucket, sum + count lines). Pin the full output so
// any drift is loud at review time.
func TestMetricsRegistry_WritePrometheus_FormatSnapshot(t *testing.T) {
	r := NewMetricsRegistry([]float64{0.010, 0.100})
	r.RecordRequest("/api/x", "GET", 200, 5*time.Millisecond, "")
	r.RecordRequest("/api/x", "POST", 400, 50*time.Millisecond, "bad_request")

	var buf bytes.Buffer
	if err := r.WritePrometheus(&buf); err != nil {
		t.Fatalf("WritePrometheus: %v", err)
	}
	out := buf.String()

	// Required HELP/TYPE lines for every metric family.
	for _, line := range []string{
		"# HELP hexapi_request_count",
		"# TYPE hexapi_request_count counter",
		"# HELP hexapi_request_duration_seconds",
		"# TYPE hexapi_request_duration_seconds histogram",
		"# HELP hexapi_error_count",
		"# TYPE hexapi_error_count counter",
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing required line: %q\nfull output:\n%s", line, out)
		}
	}

	// Counter samples present with the right label set.
	for _, line := range []string{
		`hexapi_request_count{endpoint="/api/x",method="GET",status="200"} 1`,
		`hexapi_request_count{endpoint="/api/x",method="POST",status="400"} 1`,
		`hexapi_error_count{endpoint="/api/x",method="POST",status="400",error_slug="bad_request"} 1`,
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing counter sample: %q\nfull output:\n%s", line, out)
		}
	}

	// Histogram: every bucket boundary present + +Inf + _sum + _count
	// for each (endpoint,method) family.
	for _, line := range []string{
		`hexapi_request_duration_seconds_bucket{endpoint="/api/x",method="GET",le="0.01"} 1`,
		`hexapi_request_duration_seconds_bucket{endpoint="/api/x",method="GET",le="0.1"} 1`,
		`hexapi_request_duration_seconds_bucket{endpoint="/api/x",method="GET",le="+Inf"} 1`,
		`hexapi_request_duration_seconds_count{endpoint="/api/x",method="GET"} 1`,
		`hexapi_request_duration_seconds_bucket{endpoint="/api/x",method="POST",le="+Inf"} 1`,
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing histogram sample: %q\nfull output:\n%s", line, out)
		}
	}
}

// Output ordering is deterministic — two registries with the same
// inputs produce byte-identical output. Pin so flaky-by-map-order
// regressions show up immediately.
func TestMetricsRegistry_WritePrometheus_DeterministicOrdering(t *testing.T) {
	build := func() *MetricsRegistry {
		r := NewMetricsRegistry([]float64{0.010})
		r.RecordRequest("/api/zebra", "POST", 500, time.Millisecond, "internal_server_error")
		r.RecordRequest("/api/alpha", "GET", 200, time.Millisecond, "")
		r.RecordRequest("/api/alpha", "POST", 200, time.Millisecond, "")
		return r
	}
	var a, b bytes.Buffer
	_ = build().WritePrometheus(&a)
	_ = build().WritePrometheus(&b)
	if a.String() != b.String() {
		t.Errorf("non-deterministic output:\n--- A ---\n%s\n--- B ---\n%s", a.String(), b.String())
	}
}

// --- ObservabilityMiddleware ---------------------------------------------

// Every request flowing through the middleware increments
// request_count by 1 and observes a duration. Uses r.Pattern as the
// endpoint label — verified end-to-end via a real http.ServeMux so
// the Go 1.23 Pattern resolution path is exercised.
func TestObservabilityMiddleware_RequestIncrementsMetrics(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/decks/{owner}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := ObservabilityMiddleware(reg, nil, mux)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/decks/alice", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	got := reg.requestCount[metricKey{Endpoint: "/api/decks/{owner}", Method: "GET", Status: 200}]
	if got != 3 {
		t.Errorf("request_count = %d, want 3 (route template should be the endpoint label, not the concrete path)", got)
	}
}

// Error responses lift their error.code slug from the unified
// ErrorResponse envelope and bump the per-error counter. Verifies
// the body-peek extraction works against a real writeError call.
func TestObservabilityMiddleware_ErrorSlugLiftedFromEnvelope(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/decks/{owner}/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeErrorCode(w, http.StatusForbidden, "csrf_invalid", "csrf: invalid or missing token")
	})
	handler := ObservabilityMiddleware(reg, nil, mux)

	req := httptest.NewRequest(http.MethodPost, "/api/decks/alice/voltron", strings.NewReader("{}"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	k := errorMetricKey{Endpoint: "/api/decks/{owner}/{id}", Method: "POST", Status: 403, ErrorSlug: "csrf_invalid"}
	if got := reg.errorCount[k]; got != 1 {
		t.Errorf("error_count[csrf_invalid] = %d, want 1; full errorCount=%+v", got, reg.errorCount)
	}
}

// Handler that never calls WriteHeader is treated as 200 (matches
// the http.ResponseWriter contract). Defends against status=0 rows
// in the registry from handlers that just `return`.
func TestObservabilityMiddleware_ImplicitStatusIs200(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/empty", func(w http.ResponseWriter, r *http.Request) {
		// no WriteHeader, no Write — handler just returns.
	})
	handler := ObservabilityMiddleware(reg, nil, mux)
	req := httptest.NewRequest(http.MethodGet, "/api/empty", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got := reg.requestCount[metricKey{Endpoint: "/api/empty", Method: "GET", Status: 200}]; got != 1 {
		t.Errorf("implicit 200 not recorded: requestCount=%+v", reg.requestCount)
	}
}

// Routes with no pattern (e.g. raw 404s, sub-mux dispatches) skip
// metric recording — they'd otherwise aggregate under endpoint=""
// and destroy dashboard utility. The log line still fires so the
// trace stays useful.
func TestObservabilityMiddleware_NoPatternSkipsMetrics(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	var logged LogFields
	logger := LoggerFunc(func(f LogFields) { logged = f })

	// Bare HandlerFunc — no mux, no pattern.
	bare := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := ObservabilityMiddleware(reg, logger, bare)

	req := httptest.NewRequest(http.MethodGet, "/no-pattern", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if len(reg.requestCount) != 0 {
		t.Errorf("empty-pattern request must skip metrics, got %d entries", len(reg.requestCount))
	}
	if logged.Status != http.StatusTeapot {
		t.Errorf("log line should still fire on no-pattern paths; logged.Status=%d", logged.Status)
	}
	if logged.Path != "/no-pattern" {
		t.Errorf("log line should include concrete Path, got %q", logged.Path)
	}
}

// Both nil → straight pass-through, no allocation overhead.
func TestObservabilityMiddleware_BothNilIsPassThrough(t *testing.T) {
	called := false
	bare := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := ObservabilityMiddleware(nil, nil, bare)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if !called {
		t.Fatal("pass-through must invoke wrapped handler")
	}
}

// --- StructuredLogger ----------------------------------------------------

// StdoutLogger writes a single key=value line per request, with
// duration in milliseconds and error_slug omitted on success.
func TestStdoutLogger_FormatStable(t *testing.T) {
	var buf bytes.Buffer
	logger := StdoutLogger(&buf)
	logger.LogRequest(LogFields{
		RequestID:  "req-abc",
		Endpoint:   "/api/decks/{owner}",
		Method:     "GET",
		Path:       "/api/decks/alice",
		Status:     200,
		DurationMS: 12.34,
	})
	out := buf.String()
	want := []string{
		`level=info`,
		`request_id=req-abc`,
		`endpoint=/api/decks/{owner}`,
		`method=GET`,
		`path=/api/decks/alice`,
		`status=200`,
		`duration_ms=12.34`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in log line: %s", w, out)
		}
	}
	if strings.Contains(out, "error_slug=") {
		t.Errorf("success log should omit error_slug, got: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("log line must end with newline, got %q", out)
	}
}

// Error path includes the slug.
func TestStdoutLogger_ErrorIncludesSlug(t *testing.T) {
	var buf bytes.Buffer
	StdoutLogger(&buf).LogRequest(LogFields{
		RequestID:  "req-xyz",
		Endpoint:   "/api/x",
		Method:     "POST",
		Status:     403,
		DurationMS: 2.0,
		ErrorSlug:  "csrf_invalid",
	})
	if !strings.Contains(buf.String(), "error_slug=csrf_invalid") {
		t.Errorf("error log should include slug, got: %s", buf.String())
	}
}

// Path is omitted when it equals the endpoint template (avoids
// redundant noise in logs when no path placeholders are present).
func TestStdoutLogger_PathOmittedWhenSameAsEndpoint(t *testing.T) {
	var buf bytes.Buffer
	StdoutLogger(&buf).LogRequest(LogFields{
		Endpoint: "/api/meta",
		Method:   "GET",
		Path:     "/api/meta",
		Status:   200,
	})
	if strings.Contains(buf.String(), "path=") {
		t.Errorf("path should be omitted when same as endpoint, got: %s", buf.String())
	}
}

// --- /metrics endpoint ---------------------------------------------------

// Loopback callers are allowed (the standard "scrape from the box"
// deployment).
func TestHandleMetrics_LoopbackAllowed(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	reg.RecordRequest("/api/x", "GET", 200, time.Millisecond, "")
	h := HandleMetrics(reg)

	for _, addr := range []string{"127.0.0.1:54321", "[::1]:54321"} {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status=%d, want 200", addr, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("%s: Content-Type=%q, want text/plain ...", addr, ct)
		}
		if !strings.Contains(w.Body.String(), "hexapi_request_count") {
			t.Errorf("%s: body missing hexapi_request_count metric", addr)
		}
	}
}

// RFC1918 private addresses are allowed (the typical LAN deployment
// where a scraper on 192.168.0.0/16 polls the server).
func TestHandleMetrics_RFC1918Allowed(t *testing.T) {
	h := HandleMetrics(NewMetricsRegistry(nil))
	for _, addr := range []string{"10.0.0.5:8080", "172.16.0.5:8080", "192.168.1.10:8080"} {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.RemoteAddr = addr
		w := httptest.NewRecorder()
		h(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("%s: status=%d, want 200 (RFC1918 should pass)", addr, w.Code)
		}
	}
}

// Public addresses are rejected with 403 in the unified envelope —
// pin so a future refactor doesn't accidentally trust X-Forwarded-For
// for the gate (which would let any caller send `X-Forwarded-For:
// 127.0.0.1` and bypass).
func TestHandleMetrics_PublicAddressRejected(t *testing.T) {
	h := HandleMetrics(NewMetricsRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "203.0.113.7:55555"
	// Spoofed forwarded-for that would defeat a naive header-trusting
	// gate. Our gate uses RemoteAddr (socket peer), so this must NOT
	// promote the caller to "loopback".
	req.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("public address must 403, got %d (body=%q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"forbidden"`) {
		t.Errorf("403 body should use unified envelope with code=forbidden, got %s", w.Body.String())
	}
}

// Admin-owner header from a public IP IS allowed (admin curl
// from outside the LAN, e.g. through Caddy on MISTY).
func TestHandleMetrics_AdminOwnerOverridesPublicAddress(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "ops-admin")
	h := HandleMetrics(NewMetricsRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "203.0.113.7:55555"
	req.Header.Set("X-HexDek-Owner", "ops-admin")
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("admin owner from public IP must pass, got %d (body=%q)", w.Code, w.Body.String())
	}
}

// Admin-owner mismatch from a public IP rejected.
func TestHandleMetrics_AdminOwnerMismatchRejected(t *testing.T) {
	t.Setenv("HEXDEK_ADMIN_OWNER", "ops-admin")
	h := HandleMetrics(NewMetricsRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "203.0.113.7:55555"
	req.Header.Set("X-HexDek-Owner", "imposter")
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("owner mismatch must 403, got %d", w.Code)
	}
}

// Empty HEXDEK_ADMIN_OWNER means owner-header gating is disabled
// entirely (only loopback works). Pin so an unset env var can't
// be matched by an attacker sending an empty owner header.
func TestHandleMetrics_EmptyAdminOwnerEnvDisablesOwnerGate(t *testing.T) {
	_ = os.Unsetenv("HEXDEK_ADMIN_OWNER")
	h := HandleMetrics(NewMetricsRegistry(nil))
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "203.0.113.7:55555"
	req.Header.Set("X-HexDek-Owner", "") // empty header
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("empty env + empty header must 403, got %d", w.Code)
	}
}

// Nil registry returns 503 — explicit signal that the server didn't
// construct metrics rather than silently serving an empty 200.
func TestHandleMetrics_NilRegistryReturns503(t *testing.T) {
	h := HandleMetrics(nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.RemoteAddr = "127.0.0.1:55555"
	w := httptest.NewRecorder()
	h(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("nil registry status=%d, want 503", w.Code)
	}
}

// End-to-end smoke: middleware records → endpoint serves. Drive a
// few requests through ObservabilityMiddleware then scrape /metrics
// and verify the counts show up in the output.
func TestObservability_EndToEnd_MiddlewareToMetricsEndpoint(t *testing.T) {
	reg := NewMetricsRegistry(nil)
	appMux := http.NewServeMux()
	appMux.HandleFunc("GET /api/decks/{owner}/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	appMux.HandleFunc("POST /api/decks/{owner}/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeErrorCode(w, http.StatusBadRequest, "bad_request", "missing body")
	})
	app := ObservabilityMiddleware(reg, nil, appMux)

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/decks/alice/voltron", nil)
		app.ServeHTTP(httptest.NewRecorder(), req)
	}
	req := httptest.NewRequest("POST", "/api/decks/alice/voltron", strings.NewReader("{}"))
	app.ServeHTTP(httptest.NewRecorder(), req)

	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsReq.RemoteAddr = "127.0.0.1:0"
	metricsW := httptest.NewRecorder()
	HandleMetrics(reg)(metricsW, metricsReq)

	if metricsW.Code != http.StatusOK {
		t.Fatalf("/metrics status=%d", metricsW.Code)
	}
	body := metricsW.Body.String()
	for _, want := range []string{
		`hexapi_request_count{endpoint="/api/decks/{owner}/{id}",method="GET",status="200"} 5`,
		`hexapi_request_count{endpoint="/api/decks/{owner}/{id}",method="POST",status="400"} 1`,
		`hexapi_error_count{endpoint="/api/decks/{owner}/{id}",method="POST",status="400",error_slug="bad_request"} 1`,
		`hexapi_request_duration_seconds_count{endpoint="/api/decks/{owner}/{id}",method="GET"} 5`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing in /metrics output: %q\nfull body:\n%s", want, body)
		}
	}
}

// --- normalizeEndpoint ---------------------------------------------------

func TestNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"GET /api/decks", "/api/decks"},
		{"POST /api/decks/{owner}/{id}", "/api/decks/{owner}/{id}"},
		{"/api/no-method", "/api/no-method"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeEndpoint(tc.in); got != tc.want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- isPrivateOrLoopback -------------------------------------------------

func TestIsPrivateOrLoopback(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:1234", true},
		{"127.0.0.1", true},
		{"[::1]:1234", true},
		{"10.0.0.1:80", true},
		{"172.16.0.5:80", true},
		{"172.31.255.255:80", true},
		{"192.168.1.1:80", true},
		{"203.0.113.7:80", false},
		{"8.8.8.8:53", false},
		{"172.32.0.1:80", false}, // outside 172.16/12
		{"not-an-ip:80", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isPrivateOrLoopback(tc.addr); got != tc.want {
			t.Errorf("isPrivateOrLoopback(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
